package fhs

import (
	"fmt"
	"sync"
	time "time"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"
)

const FORK = "fork"

type Fhs struct {
	node.Node
	election.Election
	pm              *pacemaker.Pacemaker
	lastVotedView   types.View
	preferredView   types.View
	bc              *blockchain.BlockChain
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	bufferedQCs     map[crypto.Identifier]*blockchain.QC
	bufferedBlocks  map[types.View]*blockchain.Block
	highQC          *blockchain.QC
	voteTimer       *time.Timer
	proposeTimer    *time.Timer
	viewChangeTimer *time.Timer
	mu              sync.Mutex
}

func NewFhs(
	node node.Node,
	pm *pacemaker.Pacemaker,
	elec election.Election,
	committedBlocks chan *blockchain.Block,
	forkedBlocks chan *blockchain.Block) *Fhs {
	f := new(Fhs)
	f.Node = node
	f.Election = elec
	f.pm = pm
	f.bc = blockchain.NewBlockchain(config.GetConfig().N())
	f.bufferedBlocks = make(map[types.View]*blockchain.Block)
	f.bufferedQCs = make(map[crypto.Identifier]*blockchain.QC)
	f.highQC = &blockchain.QC{View: 0}
	f.committedBlocks = committedBlocks
	f.forkedBlocks = forkedBlocks
	// f.pm.AdvanceView(0)

	return f
}

func (f *Fhs) ProcessBlock(block *blockchain.Block) error {
	log.Debugf("[%v] is processing block, view: %v, id: %x", f.ID(), block.View, block.ID)
	curView := f.pm.GetCurView()
	if block.Proposer != f.ID() {
		blockIsVerified, _ := crypto.PubVerify(block.Sig, crypto.IDToByte(block.ID), block.Proposer)
		if !blockIsVerified {
			log.Warningf("[%v] received a block with an invalid signature", f.ID())
		}
	}
	if block.View > curView+1 {
		//	buffer the block
		f.bufferedBlocks[block.View-1] = block
		log.Debugf("[%v] the block is buffered, view: %v, current view is: %v, id: %x", f.ID(), block.View, curView, block.ID)
		return nil
	}
	f.bc.AddBlock(block)

	log.Debugf("[%v] voting is processed, view: %v, id: %x", f.ID(), block.View, block.ID)
	shouldVote, err := f.votingRule(block)
	if err != nil {
		log.Errorf("cannot decide whether to vote the block, %w", err)
		return err
	}
	if !shouldVote {
		log.Debugf("[%v] is not going to vote for block, id: %x", f.ID(), block.ID)
		return nil
	}

	if block.QC != nil {
		f.updateHighQC(block.QC)
	} else {
		return fmt.Errorf("the block should contain a QC")
	}

	if block.Proposer != f.ID() {
		f.processCertificate(block.QC)
	}
	curView = f.pm.GetCurView()
	if !f.Election.IsLeader(block.Proposer, block.View) {
		return fmt.Errorf("received a proposal (%v) from an invalid leader (%v)", block.View, block.Proposer)
	}
	log.Debugf("[%v] is adding block to the blockchain, view: %v, id: %x", f.ID(), block.View, block.ID)

	// check commit rule
	qc := block.QC
	if qc.View >= 2 && qc.View+1 == block.View {

		ok, b, _ := f.commitRule(block)
		if ok {
			// 为了统计commit的block（根据最大的view的block统计）
			committedBlocks, forkedBlocks, err := f.bc.CommitBlock(b.ID, f.pm.GetCurView())
			var heightestBlock *blockchain.Block
			if err != nil {
				return fmt.Errorf("[%v] cannot commit blocks", f.ID())
			}

			for _, cBlock := range committedBlocks {
				f.committedBlocks <- cBlock
				if heightestBlock == nil || int(cBlock.View) > int(heightestBlock.View) {
					heightestBlock = cBlock
				}
			}
			if heightestBlock != nil {
				heightestBlock.CommitFromThis = true
			}

			for _, fBlock := range forkedBlocks {
				f.forkedBlocks <- fBlock
			}
		}

	}

	// process buffered QC
	qc, ok := f.bufferedQCs[block.ID]
	if ok {
		f.processCertificate(qc)
		delete(f.bufferedQCs, block.ID)
	}

	vote := blockchain.MakeVote(block.View, f.ID(), block.ID)
	// vote to the next leader
	voteAggregator := f.FindLeaderFor(block.View + 1)

	// 进入viewChange阶段，这个阶段计时会把view change和vote的计时器合并
	// 相当于vote阶段结束后自动进入view change阶段
	curView = f.pm.GetCurView()
	if f.proposeTimer != nil {
		f.proposeTimer.Stop()
	}
	f.voteTimer = time.AfterFunc(config.GetConfig().GetBigDelta(), func() {
		f.makeViewChangeTimer(curView)
	})

	if voteAggregator == f.ID() {
		f.ProcessVote(vote)
	} else {
		time.AfterFunc(config.GetConfig().GetTrueDelay(), func() {
			f.Send(voteAggregator, vote)
		})

	}
	log.Debugf("[%v] vote is sent, id: %x", f.ID(), vote.BlockID)
	f.enterNewView(block.View - 1)
	b, ok := f.bufferedBlocks[block.View]
	if ok {
		err := f.ProcessBlock(b)
		return err
	}

	return nil
}

func (f *Fhs) ProcessVote(vote *blockchain.Vote) {

	// if f.IsByz() && config.GetConfig().SilentATK {
	// 	return
	// }

	log.Debugf("[%v] is processing the vote from %v, block id: %x, view %v", f.ID(), vote.Voter, vote.BlockID, vote.View)
	if f.ID() != vote.Voter {
		voteIsVerified, err := crypto.PubVerify(vote.Signature, crypto.IDToByte(vote.BlockID), vote.Voter)
		if err != nil {
			log.Fatalf("[%v] Error in verifying the signature in vote id: %x", f.ID(), vote.BlockID)
			return
		}
		if !voteIsVerified {
			log.Warningf("[%v] received a vote with unvalid signature. vote id: %x", f.ID(), vote.BlockID)
			return
		}
	}
	block, _ := f.bc.GetBlockByID(vote.BlockID)

	isBuilt, qc := f.bc.AddVote(vote)
	if !isBuilt {
		log.Debugf("[%v] not sufficient votes to build a QC, block id: %x", f.ID(), vote.BlockID)
		return
	}
	qc.Leader = f.ID()
	_, err := f.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		f.bufferedQCs[qc.BlockID] = qc
		return
	}

	ok, _, _ := f.commitRule(block)

	if !ok && config.GetConfig().SilentATK && f.IsByz() {
		f.processCertificate(qc)
	}

	if f.IsByz() && config.GetConfig().ForkATK && !block.Mali {

		return
	}

	f.processCertificate(qc)
}

func (f *Fhs) ProcessRemoteTmo(tmo *pacemaker.TMO) {
	log.Debugf("[%v] is processing tmo from %v", f.ID(), tmo.NodeID)
	if tmo.View < f.pm.GetCurView() {
		return
	}
	isBuilt, tc := f.pm.ProcessRemoteTmo(tmo)
	if !isBuilt {
		log.Debugf("[%v] not enough tc for %v", f.ID(), tmo.View)
		return
	}
	log.Debugf("[%v] a tc is built for view %v", f.ID(), tc.View)
	f.processTC(tc)
}

func (f *Fhs) ProcessLocalTmo(view types.View) {
}

func (f *Fhs) ProcessRemoteVmo(vmo *pacemaker.VMO) {

}

func (f *Fhs) ProcessVMOAndBlock(view types.View) bool {

	return true
}

// send a vmo to other replicas, and process the vmo locally

func (f *Fhs) ProcessLocalVmo(view types.View) {

}

func (f *Fhs) ProcessRemoteVc(vc *pacemaker.VC) {

}

func (f *Fhs) enterNewView(view types.View) {
	if view < f.pm.GetCurView() {
		return
	}
	log.Debugf("[%v] is try entering new view %v", f.ID(), view)
	if f.proposeTimer != nil {
		f.proposeTimer.Stop()
	}
	if f.voteTimer != nil {
		f.voteTimer.Stop()
	}
	if f.viewChangeTimer != nil {
		f.viewChangeTimer.Stop()
	}

	f.pm.AdvanceView(view)
	log.Debugf("[%v] set ProposeTimer %v", f.ID(), view)
	f.proposeTimer = time.AfterFunc(config.GetConfig().GetBigDelta(), func() {
		f.makeViewChangeTimer(view + 1)
	})

}

func (f *Fhs) makeViewChangeTimer(view types.View) {
	f.viewChangeTimer = time.AfterFunc(config.GetConfig().GetBigDelta(), func() {
		f.enterNewView(view)
	})
}

func (f *Fhs) MakeProposal(view types.View, payload []*message.Transaction) *blockchain.Block {
	// if f.IsByz() && config.GetConfig().SilentATK {
	// 	return nil
	// }
	qc, forkNum := f.forkChoice()
	block := blockchain.MakeBlock(view, qc, qc.BlockID, payload, f.ID(), f.IsByz(), forkNum, 0)
	ok, _, _ := f.commitRule(block)
	if f.IsByz() && config.GetConfig().SilentATK {
		if !ok {
			time.Sleep(config.GetConfig().GetBigDelta() / 10 * 16)
			for i := 1; i <= config.GetConfig().ByzNo/3; i++ {
				nodeId := identity.NewNodeID(i + config.GetConfig().ByzNo)
				f.Send(nodeId, block)
			}
			return block
		}
		return nil
	}
	time.Sleep(config.GetConfig().GetTrueDelay())
	log.Debugf("[%v] is making a proposal for view %v, forkNum: %v, qc's view %v", f.ID(), view, forkNum, qc.View)

	return block
}

func (f *Fhs) forkChoice() (*blockchain.QC, int) {
	choice := f.GetHighQC()
	forkNum := 0
	if f.IsByz() {

		if f.FindLeaderFor(f.pm.GetCurView()+1).Node() > config.GetConfig().ByzNo {
			forkNum = 1
		}

		return choice, forkNum
	} else {

		return choice, forkNum
	}
	// to simulate TC under forking attack
}

func (f *Fhs) processTC(tc *pacemaker.TC) {
	if tc.View < f.pm.GetCurView() {
		return
	}
	f.pm.AdvanceView(tc.View)
}

func (f *Fhs) GetChainStatus() string {
	chainGrowthRate := f.bc.GetChainGrowth()
	blockIntervals := f.bc.GetBlockIntervals()
	return fmt.Sprintf("[%v] The current view is: %v, chain growth rate is: %v, ave block interval is: %v", f.ID(), f.pm.GetCurView(), chainGrowthRate, blockIntervals)
}

func (f *Fhs) GetHighQC() *blockchain.QC {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.highQC
}

func (f *Fhs) updateHighQC(qc *blockchain.QC) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if qc.View > f.highQC.View {
		f.highQC = qc
	}
}

func (f *Fhs) processCertificate(qc *blockchain.QC) {
	log.Debugf("[%v] is processing a QC, block id: %x", f.ID(), qc.BlockID)

	if qc.Leader != f.ID() {
		quorumIsVerified, _ := crypto.VerifyQuorumSignature(qc.AggSig, qc.BlockID, qc.Signers)
		if quorumIsVerified == false {
			log.Warningf("[%v] received a quorum with invalid signatures", f.ID())
			return
		}
	}

	err := f.updatePreferredView(qc)
	if err != nil {
		f.bufferedQCs[qc.BlockID] = qc
		log.Debugf("[%v] a qc is buffered, view: %v, id: %x", f.ID(), qc.View, qc.BlockID)
		return
	}
	f.updateHighQC(qc)
	if f.IsByz() && config.GetConfig().SilentATK {
		return
	}
	f.enterNewView(qc.View)
}

func (f *Fhs) votingRule(block *blockchain.Block) (bool, error) {
	if block.View <= 2 {
		return true, nil
	}
	parentBlock, err := f.bc.GetParentBlock(block.ID)
	if err != nil {
		log.Debugf("[%v] cannot vote for block: %w, %v, %v", f.ID(), err, block.QC.View, block.QC.Leader)
		return false, fmt.Errorf("cannot vote for block: %w, %v, %v", err, block.QC.View, block.QC.Leader)
	}
	if (block.View <= f.lastVotedView) || (parentBlock.View < f.preferredView) {
		if parentBlock.View < f.preferredView {
			log.Debugf("[%v] parent block view is: %v and preferred view is: %v", f.ID(), parentBlock.View, f.preferredView)
		}
		return false, nil
	}
	return true, nil
}

func (f *Fhs) commitRule(block *blockchain.Block) (bool, *blockchain.Block, error) {
	qc := block.QC
	parentBlock, err := f.bc.GetParentBlock(qc.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	if (parentBlock.View + 1) == qc.View {
		return true, parentBlock, nil
	}
	return false, nil, nil
}

func (f *Fhs) updateLastVotedView(targetView types.View) error {
	if targetView < f.lastVotedView {
		return fmt.Errorf("target view is lower than the last voted view")
	}
	f.lastVotedView = targetView
	return nil
}

func (f *Fhs) updatePreferredView(qc *blockchain.QC) error {
	if qc.View < 2 {
		return nil
	}
	_, err := f.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	if qc.View > f.preferredView {
		log.Debugf("[%v] preferred view has been updated to %v", f.ID(), qc.View)
		f.preferredView = qc.View
	}
	return nil
}
