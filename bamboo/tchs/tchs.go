package tchs

import (
	"fmt"
	"sync"
	"time"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"
)

const FORK = "fork"

type Tchs struct {
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
	recivedVMO      map[types.View]*pacemaker.VMO
	recivedBlock    map[types.View]*blockchain.Block
	highQC          *blockchain.QC
	mu              sync.Mutex
	voteTimer       *time.Timer
	proposeTimer    *time.Timer
	viewChangeTimer *time.Timer
}

func NewTchs(
	node node.Node,
	pm *pacemaker.Pacemaker,
	elec election.Election,
	committedBlocks chan *blockchain.Block,
	forkedBlocks chan *blockchain.Block) *Tchs {
	th := new(Tchs)
	th.Node = node
	th.Election = elec
	th.pm = pm
	th.bc = blockchain.NewBlockchain(config.GetConfig().N())
	th.bufferedBlocks = make(map[types.View]*blockchain.Block)
	th.bufferedQCs = make(map[crypto.Identifier]*blockchain.QC)
	th.recivedVMO = make(map[types.View]*pacemaker.VMO)
	th.recivedBlock = make(map[types.View]*blockchain.Block)
	th.highQC = &blockchain.QC{View: 0}
	th.committedBlocks = committedBlocks
	th.forkedBlocks = forkedBlocks
	th.pm.AdvanceView(0)
	return th
}

func (th *Tchs) ProcessBlock(block *blockchain.Block) error {
	log.Debugf("[%v] is processing block, view: %v, id: %x", th.ID(), block.View, block.ID)
	// only leader start with 1 view， so must let other replicas to start with 1 view
	curView := th.pm.GetCurView()
	if block.Proposer != th.ID() {
		blockIsVerified, _ := crypto.PubVerify(block.Sig, crypto.IDToByte(block.ID), block.Proposer)
		if !blockIsVerified {
			log.Warningf("[%v] received a block with an invalid signature", th.ID())
		}
	}
	if block.View > curView {
		//	buffer the block
		th.bufferedBlocks[block.View-1] = block
		log.Debugf("[%v] the block is buffered, view: %v, current view is: %v, id: %x", th.ID(), block.View, curView, block.ID)
		return nil
	} else if th.getBufferedBlock(block.View) != nil {
		th.deleteBufferedBlock(block.View)

	}

	if block.QC != nil {
		th.updateHighQC(block.QC)
	} else {
		return fmt.Errorf("the block should contain a QC")
	}
	if block.Proposer != th.ID() {
		th.processCertificate(block.QC)
	}

	if !th.Election.IsLeader(block.Proposer, block.View) {
		return fmt.Errorf("received a proposal (%v) from an invalid leader (%v)", block.View, block.Proposer)
	}

	th.bc.AddBlock(block)

	// check commit rule
	qc := block.QC
	if qc.View >= 2 && qc.View+1 == block.View {
		ok, b, _ := th.commitRule(block)
		if !ok {
			// return nil
		} else {
			committedBlocks, forkedBlocks, err := th.bc.CommitBlock(b.ID, th.pm.GetCurView())
			var heightestBlock *blockchain.Block

			if err != nil {
				return fmt.Errorf("[%v] cannot commit blocks", th.ID())
			}
			for _, cBlock := range committedBlocks {
				th.committedBlocks <- cBlock
				if heightestBlock == nil || int(cBlock.View) > int(heightestBlock.View) {
					heightestBlock = cBlock
				}
			}
			if heightestBlock != nil {
				heightestBlock.CommitFromThis = true
			}
			for _, fBlock := range forkedBlocks {
				th.forkedBlocks <- fBlock
			}
		}

	}
	th.recivedBlock[block.View-1] = block
	th.ProcessVMOAndBlock(block.View)
	// process buffered QC
	qc, ok := th.bufferedQCs[block.ID]
	if ok {
		th.processCertificate(qc)
		delete(th.bufferedQCs, block.ID)
		if th.FindLeaderFor(block.View+1) == th.ID() {
			th.ProcessLocalVmo(block.View)
		}
	}

	shouldVote, err := th.votingRule(block)
	if err != nil {
		// log.Errorf("cannot decide whether to vote the block, %w", err)
		return err
	}
	if !shouldVote {
		log.Debugf("[%v] is not going to vote for block, id: %x", th.ID(), block.ID)
		return nil
	}
	vote := blockchain.MakeVote(block.View, th.ID(), block.ID)
	// vote to the next leader
	voteAggregator := th.FindLeaderFor(block.View + 1)
	if voteAggregator == th.ID() {
		th.ProcessVote(vote)
	} else {
		time.AfterFunc(config.GetConfig().GetTrueDelay(), func() {
			th.Send(voteAggregator, vote)
		})

	}
	log.Debugf("[%v] vote is sent, id: %x", th.ID(), vote.BlockID)

	b, ok := th.bufferedBlocks[block.View]
	if ok {
		err := th.ProcessBlock(b)
		return err
	}

	return nil
}

func (th *Tchs) ProcessVote(vote *blockchain.Vote) {
	if th.IsByz() && config.GetConfig().SilentATK {
		return
	}
	log.Debugf("[%v] is processing the vote from %v, block id: %x", th.ID(), vote.Voter, vote.BlockID)
	if th.ID() != vote.Voter {
		voteIsVerified, err := crypto.PubVerify(vote.Signature, crypto.IDToByte(vote.BlockID), vote.Voter)
		if err != nil {
			log.Fatalf("[%v] Error in verifying the signature in vote id: %x", th.ID(), vote.BlockID)
			return
		}
		if !voteIsVerified {
			log.Warningf("[%v] received a vote with unvalid signature. vote id: %x", th.ID(), vote.BlockID)
			return
		}
	}
	isBuilt, qc := th.bc.AddVote(vote)
	if !isBuilt {
		log.Debugf("[%v] not sufficient votes to build a QC, block id: %x", th.ID(), vote.BlockID)
		return
	}
	qc.Leader = th.ID()
	_, err := th.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		th.bufferedQCs[qc.BlockID] = qc
		return
	}
	th.processCertificate(qc)
	th.ProcessLocalVmo(th.pm.GetCurView())
}

func (th *Tchs) ProcessRemoteTmo(tmo *pacemaker.TMO) {
	log.Debugf("[%v] is processing tmo from %v", th.ID(), tmo.NodeID)
	log.Debugf("current view is %v, tmo is %v", th.pm.GetCurView(), tmo.View)
	if tmo.View < th.pm.GetCurView() {
		return
	}
	isBuilt, tc := th.pm.ProcessRemoteTmo(tmo)
	if !isBuilt {
		log.Debugf("[%v] not enough tc for %v", th.ID(), tmo.View)
		return
	}
	log.Debugf("[%v] a tc is built for view %v", th.ID(), tc.View)
	th.processTC(tc)
}

func (th *Tchs) ProcessLocalTmo(view types.View) {
	tmo := &pacemaker.TMO{
		View:   view,
		NodeID: th.ID(),
		HighQC: th.GetHighQC(),
	}
	th.Broadcast(tmo)
	th.ProcessRemoteTmo(tmo)
	log.Debugf("[%v] broadcast is done for sending tmo", th.ID())
}

func (th *Tchs) ProcessRemoteVmo(vmo *pacemaker.VMO) {
	nextLeaderId := th.FindLeaderFor(vmo.View + 1)
	if nextLeaderId == th.ID() {
		time.Sleep(config.GetConfig().GetBigDelta())
		th.pm.AdvanceView(vmo.View)
		th.MakeProposalTimer(vmo.View + 1)
	}
	// if the replica is not the next leader, send the vmo to the next leader,
	// and try to change the view when vmo and block are received
	if nextLeaderId != th.ID() {
		// log.Debugf("[%v] is processing vmo, view: %v, from: %v, current view is %v", th.ID(), vmo.View, vmo.NodeID, th.pm.GetCurView())
		th.recivedVMO[vmo.View-1] = vmo
		if th.pm.GetCurView() < vmo.View {
			return
		}
		ok := th.ProcessVMOAndBlock(vmo.View)
		// recusive call
		if ok {
			vmo, ok := th.recivedVMO[vmo.View]
			if ok {
				th.ProcessRemoteVmo(vmo)
			}
		}
	}
}

func (th *Tchs) ProcessVMOAndBlock(view types.View) bool {
	vmo, ok := th.recivedVMO[view-1]
	if !ok {
		return false
	}
	if view < th.pm.GetCurView() {
		return false
	}
	block := th.getBufferedBlock(view)
	if block != nil {
		th.ProcessBlock(block)
	}
	_, ok = th.recivedBlock[view-1]
	if !ok {
		return false
	}
	// if block haven't been processed(just buffer), process the block

	nextLeaderId := th.FindLeaderFor(view + 1)
	vmo.NodeID = th.ID()
	if nextLeaderId == th.ID() {
		time.Sleep(config.GetConfig().GetBigDelta())
	}
	th.pm.AdvanceView(view)
	th.MakeViewChangeTimer(th.pm.GetCurView())

	delete(th.recivedVMO, view-1)
	th.deleteRecivedBlock(view)

	return true
}

// send a vmo to other replicas, and process the vmo locally

func (th *Tchs) ProcessLocalVmo(view types.View) {
	vmo := &pacemaker.VMO{
		View:   view,
		NodeID: th.ID(),
		HighQC: th.GetHighQC(),
	}
	log.Debugf("[%v] is broadcast vmo, view: %v", th.ID(), view)
	time.AfterFunc(time.Duration(config.GetConfig().GetTrueDelay()), func() {
		th.Broadcast(vmo)
	})

	th.ProcessRemoteVmo(vmo)

}

func (th *Tchs) ProcessRemoteVc(vc *pacemaker.VC) {
	th.pm.AdvanceView(vc.View)
	th.MakeProposalTimer(th.pm.GetCurView())
}

func (th *Tchs) MakeProposal(view types.View, payload []*message.Transaction) *blockchain.Block {
	time.Sleep(time.Duration(config.GetConfig().GetTrueDelay()))
	if th.IsByz() && config.GetConfig().SilentATK {
		return nil
	}

	qc, flag, _ := th.forkChoice()
	block := blockchain.MakeBlock(view, qc, qc.BlockID, payload, th.ID(), th.IsByz(), flag, 0)
	return block
}

func (th *Tchs) forkChoice() (*blockchain.QC, int, int) {
	if th.IsByz() && config.Configuration.ForkATK {
		return th.forkRule()
	}
	// to simulate TC under forking attack

	return th.GetHighQC(), 0, 0
}

func (th *Tchs) MakeProposalTimer(view types.View) {
	times := config.GetConfig().GetBigDelta()
	if !th.IsLeader(th.ID(), view) {
		times = times * 2
	}
	th.proposeTimer = time.AfterFunc(times, func() {

		if th.getRecivedBlock(view) == nil && th.getBufferedBlock(view) == nil {
			log.Debugf("[%v] proposal out of time, view: %v", th.ID(), view)
			th.processTimeOutVmo(view)
		} else {
			log.Debugf("[%v] proposal success, view: %v", th.ID(), view)
			// 进行投票阶段
			th.MakeVoteTimer(view)
		}
	})
}

func (th *Tchs) MakeVoteTimer(view types.View) {
	if th.pm.GetCurView() != view {
		return
	}
	if th.proposeTimer != nil {
		th.proposeTimer.Stop()
	}
	th.voteTimer = time.AfterFunc(config.GetConfig().GetBigDelta(), func() {
		if th.pm.GetCurView() == view {
			log.Debugf("[%v] vote out of time, view: %v", th.ID(), view)
			th.processTimeOutVmo(view)
		}
	})

}

func (th *Tchs) MakeViewChangeTimer(view types.View) {
	// 跟proposal timer合并了

	th.MakeProposalTimer(view)

}

func (th *Tchs) processTimeOutVmo(view types.View) {
	nextLeaderId := th.FindLeaderFor(view + 1)
	if nextLeaderId == th.ID() {
		time.Sleep(config.GetConfig().GetBigDelta())
	}
	th.pm.AdvanceView(view)
	block := th.getBufferedBlock(view + 1)
	if block != nil {
		th.ProcessBlock(block)
	} else {
		th.MakeViewChangeTimer(th.pm.GetCurView())
		log.Debugf("[%v] is not buffer: %v", th.ID(), view)
	}
}

func (th *Tchs) forkRule() (*blockchain.QC, int, int) {
	var flag int = 0
	var height int = 0
	var choice *blockchain.QC
	//罗要fork的block的parent block
	parBlockID := th.GetHighQC().BlockID
	parBlock, err := th.bc.GetBlockByID(parBlockID)
	if err != nil {
		log.Warningf("cannot get parent block of block id: %x: %w", parBlockID, err)
	}
	if parBlock.View < th.preferredView || parBlock.GetMali() {
		flag = 0
		choice = th.GetHighQC()
		height = 0
		return choice, flag, height
	} else {
		flag = 1
		choice = parBlock.QC
		height = parBlock.GetHeight()
	}

	// to simulate Tc's view
	choice.View = th.pm.GetCurView() - 1
	return choice, flag, height
}

func (th *Tchs) processTC(tc *pacemaker.TC) {
	if tc.View < th.pm.GetCurView() {
		return
	}
	th.pm.AdvanceView(tc.View)
}

func (th *Tchs) GetChainStatus() string {
	chainGrowthRate := th.bc.GetChainGrowth()
	blockIntervals := th.bc.GetBlockIntervals()
	return fmt.Sprintf("[%v] The current view is: %v, chain growth rate is: %v, ave block interval is: %v", th.ID(), th.pm.GetCurView(), chainGrowthRate, blockIntervals)
}

func (th *Tchs) GetHighQC() *blockchain.QC {
	th.mu.Lock()
	defer th.mu.Unlock()
	return th.highQC
}

func (th *Tchs) updateHighQC(qc *blockchain.QC) {
	th.mu.Lock()
	defer th.mu.Unlock()
	if qc.View > th.highQC.View {
		th.highQC = qc
	}
}

func (th *Tchs) processCertificate(qc *blockchain.QC) {
	log.Debugf("[%v] is processing a QC, block id: %x", th.ID(), qc.BlockID)
	if qc.Leader != th.ID() {
		quorumIsVerified, _ := crypto.VerifyQuorumSignature(qc.AggSig, qc.BlockID, qc.Signers)
		if quorumIsVerified == false {
			log.Warningf("[%v] received a quorum with invalid signatures", th.ID())
			return
		}
	}
	err := th.updatePreferredView(qc)
	if err != nil {
		th.bufferedQCs[qc.BlockID] = qc
		log.Debugf("[%v] a qc is buffered, view: %v, id: %x", th.ID(), qc.View, qc.BlockID)
		return
	}
	th.updateHighQC(qc)
}

func (th *Tchs) votingRule(block *blockchain.Block) (bool, error) {
	if block.View <= 2 {
		return true, nil
	}
	parentBlock, err := th.bc.GetParentBlock(block.ID)
	if err != nil {
		return false, fmt.Errorf("cannot vote for block: %w", err)
	}
	if (block.View <= th.lastVotedView) || (parentBlock.View < th.preferredView) {
		if parentBlock.View < th.preferredView {
			log.Debugf("[%v] parent block view is: %v and preferred view is: %v", th.ID(), parentBlock.View, th.preferredView)
		}
		return false, nil
	}
	return true, nil
}

func (th *Tchs) commitRule(block *blockchain.Block) (bool, *blockchain.Block, error) {
	qc := block.QC
	parentBlock, err := th.bc.GetParentBlock(qc.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	if (parentBlock.View + 1) == qc.View {
		return true, parentBlock, nil
	}
	return false, nil, nil
}

func (th *Tchs) updateLastVotedView(targetView types.View) error {
	if targetView < th.lastVotedView {
		return fmt.Errorf("target view is lower than the last voted view")
	}
	th.lastVotedView = targetView
	return nil
}

func (th *Tchs) updatePreferredView(qc *blockchain.QC) error {
	if qc.View < 2 {
		return nil
	}

	parentBlock, err := th.bc.GetParentBlock(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	if parentBlock.View > th.preferredView {
		log.Debugf("[%v] preferred view has been updated to %v", th.ID(), qc.View)
		th.preferredView = parentBlock.View
	}
	return nil
}
func (th *Tchs) setBufferedBlock(view types.View, block *blockchain.Block) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.bufferedBlocks[view-1] = block
}

func (th *Tchs) deleteBufferedBlock(view types.View) {
	th.mu.Lock()
	defer th.mu.Unlock()
	if th.bufferedBlocks[view] != nil {
		delete(th.bufferedBlocks, view-1)
	}
}

func (th *Tchs) getBufferedBlock(view types.View) *blockchain.Block {
	th.mu.Lock()
	defer th.mu.Unlock()
	return th.bufferedBlocks[view-1]
}

func (th *Tchs) getRecivedBlock(view types.View) *blockchain.Block {
	th.mu.Lock()
	defer th.mu.Unlock()
	return th.recivedBlock[view-1]
}

func (th *Tchs) setRecivedBlock(view types.View, block *blockchain.Block) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.recivedBlock[view-1] = block
}

func (th *Tchs) deleteRecivedBlock(view types.View) {
	th.mu.Lock()
	defer th.mu.Unlock()
	if th.recivedBlock[view] != nil {
		delete(th.recivedBlock, view-1)
	}
}
