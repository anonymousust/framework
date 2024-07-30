package hotstuff

import (
	"fmt"
	"sync"
	"time"

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

type HotStuff struct {
	node.Node
	election.Election
	pm              *pacemaker.Pacemaker
	preferredView   types.View
	highQC          *blockchain.QC
	height          int
	bc              *blockchain.BlockChain
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	bufferedQCs     map[crypto.Identifier]*blockchain.QC
	bufferedBlocks  map[types.View]*blockchain.Block
	recivedVMO      map[types.View]*pacemaker.VMO
	recivedBlock    map[types.View]*blockchain.Block
	voteTimer       *time.Timer
	proposeTimer    *time.Timer
	viewChangeTimer *time.Timer

	mu sync.Mutex
}

func NewHotStuff(
	node node.Node,
	pm *pacemaker.Pacemaker,
	elec election.Election,
	committedBlocks chan *blockchain.Block,
	forkedBlocks chan *blockchain.Block) *HotStuff {
	hs := new(HotStuff)
	hs.Node = node
	hs.Election = elec
	hs.pm = pm
	hs.bc = blockchain.NewBlockchain(config.GetConfig().N())
	hs.bufferedBlocks = make(map[types.View]*blockchain.Block)
	hs.bufferedQCs = make(map[crypto.Identifier]*blockchain.QC)
	hs.recivedVMO = make(map[types.View]*pacemaker.VMO)
	hs.recivedBlock = make(map[types.View]*blockchain.Block)
	hs.highQC = &blockchain.QC{View: 0}
	hs.committedBlocks = committedBlocks
	hs.forkedBlocks = forkedBlocks
	hs.pm.AdvanceView(0)
	return hs
}

func (hs *HotStuff) ProcessBlock(block *blockchain.Block) error {

	if block == nil {
		return nil
	}
	_, err := hs.bc.GetBlockByID(block.ID)
	if err == nil {
		log.Debugf("[%v] received a block that has been processed, id: %x", hs.ID(), block.ID)
		return nil
	}

	log.Debugf("[%v] is processing block from %v, view: %v, id: %x", hs.ID(), block.Proposer.Node(), block.View, block.ID)
	curView := hs.pm.GetCurView()
	if block.Proposer != hs.ID() {
		blockIsVerified, _ := crypto.PubVerify(block.Sig, crypto.IDToByte(block.ID), block.Proposer)
		if !blockIsVerified {
			log.Warningf("[%v] received a block with an invalid signature", hs.ID())
		}
	}
	if block.View > curView {
		//	buffer the block
		hs.setBufferedBlock(block.View, block)
		log.Debugf("[%v] the block is buffered, id: %x", hs.ID(), block.ID)
		return nil
	} else if hs.getBufferedBlock(block.View) != nil {
		hs.deleteBufferedBlock(block.View)
	}
	if hs.proposeTimer != nil {
		hs.proposeTimer.Stop()
	}

	if block.QC != nil {
		hs.updateHighQC(block.QC)
	} else {
		return fmt.Errorf("the block should contain a QC")
	}

	// does not have to process the QC if the replica is the proposer
	if block.Proposer != hs.ID() {
		log.Debugf("[%v] is processing a QC from %v, view: %v, id: %x", hs.ID(), block.Proposer.Node(), block.View, block.QC.BlockID)
		hs.processCertificate(block.QC)
	}
	if !hs.Election.IsLeader(block.Proposer, block.View) {
		return fmt.Errorf("received a proposal (%v) from an invalid leader (%v)", block.View, block.Proposer)
	}
	hs.bc.AddBlock(block)
	if block.View < curView {
		return nil
	}
	// process buffered QC
	qc, ok := hs.bufferedQCs[block.ID]
	if ok {
		log.Debugf("[%v] is processing a buffered QC, block id: %x", hs.ID(), block.ID)
		hs.processCertificate(qc)
		delete(hs.bufferedQCs, block.ID)
		if hs.FindLeaderFor(block.View+1) == hs.ID() {
			hs.ProcessLocalVmo(block.View)
		}
	}
	hs.setRecivedBlock(block.View, block)
	hs.ProcessVMOAndBlock(block.View)
	// 进入投票阶段
	hs.MakeVoteTimer(hs.pm.GetCurView())

	shouldVote, err := hs.votingRule(block)
	if err != nil {
		// log.Errorf("[%v] cannot decide whether to vote the block %w, %v", hs.ID(), err, block.View)
		return err
	}
	if !shouldVote {
		log.Debugf("[%v] is not going to vote for block, id: %x", hs.ID(), block.ID)
		return nil
	}
	vote := blockchain.MakeVote(block.View, hs.ID(), block.ID)
	// vote is sent to the next leader
	voteAggregator := hs.FindLeaderFor(block.View + 1)
	if voteAggregator == hs.ID() {
		log.Debugf("[%v] vote is sent to itself, id: %x", hs.ID(), vote.BlockID)
		hs.ProcessVote(vote)
	} else {
		log.Debugf("[%v] vote is sent to %v, id: %x", hs.ID(), voteAggregator, vote.BlockID)
		time.AfterFunc(config.Configuration.GetTrueDelay(), func() {
			hs.Send(voteAggregator, vote)
		})

	}

	b := hs.getBufferedBlock(block.View + 1)
	if b != nil {
		_ = hs.ProcessBlock(b)

	}

	return nil
}

func (hs *HotStuff) ProcessVote(vote *blockchain.Vote) {
	log.Debugf("[%v] is processing the vote, block id: %x", hs.ID(), vote.BlockID)
	if vote.Voter != hs.ID() {
		voteIsVerified, err := crypto.PubVerify(vote.Signature, crypto.IDToByte(vote.BlockID), vote.Voter)
		if err != nil {
			log.Warningf("[%v] Error in verifying the signature in vote id: %x", hs.ID(), vote.BlockID)
			return
		}
		if !voteIsVerified {
			log.Warningf("[%v] received a vote with invalid signature. vote id: %x", hs.ID(), vote.BlockID)
			return
		}
	}
	isBuilt, qc := hs.bc.AddVote(vote)
	if !isBuilt {
		log.Debugf("[%v] not sufficient votes to build a QC, block id: %x", hs.ID(), vote.BlockID)
		return
	}
	qc.Leader = hs.ID()
	// buffer the QC if the block has not been received

	if hs.IsByz() && config.GetConfig().SilentATK {
		return
	}
	_, err := hs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		hs.bufferedQCs[qc.BlockID] = qc
		return
	}
	hs.processCertificate(qc)
	// 如果是byz，不会通知进入view change
	if hs.IsByz() {
		return
	}
	hs.ProcessLocalVmo(hs.pm.GetCurView())
}

func (hs *HotStuff) ProcessRemoteTmo(tmo *pacemaker.TMO) {
	log.Debugf("[%v] is processing tmo from %v", hs.ID(), tmo.NodeID)
	hs.processCertificate(tmo.HighQC)
	isBuilt, tc := hs.pm.ProcessRemoteTmo(tmo)
	if !isBuilt {
		return
	}
	log.Debugf("[%v] a tc is built for view %v", hs.ID(), tc.View)
	hs.processTC(tc)
}

func (hs *HotStuff) ProcessLocalTmo(view types.View) {
	tmo := &pacemaker.TMO{
		View:   view,
		NodeID: hs.ID(),
		HighQC: hs.GetHighQC(),
	}
	hs.Broadcast(tmo)
	hs.ProcessRemoteTmo(tmo)
}

func (hs *HotStuff) ProcessRemoteVmo(vmo *pacemaker.VMO) {

	nextLeaderId := hs.FindLeaderFor(vmo.View + 1)
	// if the replica is the next leader， try to build a vc to change the view
	if nextLeaderId == hs.ID() {
		isBuilt, vc := hs.pm.ProcessRemoteVmo(vmo)
		if !isBuilt {
			return
		}
		hs.ProcessRemoteVc(vc)
	}
	// if the replica is not the next leader, send the vmo to the next leader,
	// and try to change the view when vmo and block are received
	if nextLeaderId != hs.ID() {
		log.Debugf("[%v] is processing vmo, view: %v, from: %v, current view is %v", hs.ID(), vmo.View, vmo.NodeID, hs.pm.GetCurView())
		hs.recivedVMO[vmo.View-1] = vmo
		if hs.pm.GetCurView() < vmo.View {
			return
		}
		ok := hs.ProcessVMOAndBlock(vmo.View)
		// recusive call
		if ok {
			vmo, ok := hs.recivedVMO[vmo.View]
			if ok {
				hs.ProcessRemoteVmo(vmo)
			}
		}
	}
}

func (hs *HotStuff) ProcessVMOAndBlock(view types.View) bool {
	vmo, ok := hs.recivedVMO[view-1]
	if !ok {
		return false
	}
	if view < hs.pm.GetCurView() {
		return false
	}
	block := hs.getBufferedBlock(view)
	if block != nil {
		hs.ProcessBlock(block)
	}
	ok = hs.getRecivedBlock(view) != nil
	if !ok {
		return false
	}
	// if block haven't been processed, process the block

	nextLeaderId := hs.FindLeaderFor(view + 1)
	vmo.NodeID = hs.ID()
	if nextLeaderId != hs.ID() {
		log.Debugf("[%v] is postback vmo, view: %v", hs.ID(), view)
		time.Sleep(config.GetConfig().GetTrueDelay())
		hs.Send(nextLeaderId, vmo)
	}
	delete(hs.recivedVMO, view-1)
	hs.deleteRecivedBlock(view)

	// 处理从leader发来的vmo，直接进入newView并进入view change阶段
	hs.pm.AdvanceView(view)
	hs.MakeProposalTimer(hs.pm.GetCurView())
	return true
}

// send a vmo to other replicas, and process the vmo locally

func (hs *HotStuff) ProcessLocalVmo(view types.View) {
	vmo := &pacemaker.VMO{
		View:   view,
		NodeID: hs.ID(),
		HighQC: hs.GetHighQC(),
	}
	log.Debugf("[%v] is broadcast vmo, view: %v", hs.ID(), view)
	hs.ProcessRemoteVmo(vmo)

	hs.Broadcast(vmo)

}

func (hs *HotStuff) ProcessRemoteVc(vc *pacemaker.VC) {
	if hs.IsByz() {
		return
	}

	hs.pm.AdvanceView(vc.View)
	block := hs.getBufferedBlock(vc.View + 1)
	if block != nil {
		hs.ProcessBlock(block)
	}
	if hs.viewChangeTimer != nil {
		hs.viewChangeTimer.Stop()
	}
	hs.MakeProposalTimer(hs.pm.GetCurView())
}

func (hs *HotStuff) MakeProposal(view types.View, payload []*message.Transaction) *blockchain.Block {

	qc, forkNum, height := hs.forkChoice()
	block := blockchain.MakeBlock(view, qc, qc.BlockID, payload, hs.ID(), hs.IsByz(), forkNum, height)
	ok, _, _ := hs.commitRule(qc)
	// 打补丁，如果用的是第一个块（没父区块进行fork后续判断会有问题，这里拦掉）
	if hs.IsByz() && qc.View == 0 {
		return nil
	}

	if hs.IsByz() && config.GetConfig().SilentATK {
		if !ok {
			time.Sleep(config.GetConfig().GetBigDelta() / 20 * 15)
			// 发给f个节点
			for i := 1; i <= config.GetConfig().N()/2; i++ {
				nodeId := identity.NewNodeID(i)
				hs.Send(nodeId, block)
			}
			return block
		}
		return nil
	}
	if hs.IsByz() {
		// fork的时候慢出块
		time.Sleep(config.GetConfig().GetBigDelta() * 40 / 50)
	} else {
		time.Sleep(config.GetConfig().GetTrueDelay())
	}

	return block
}

func (hs *HotStuff) MakeProposalTimer(view types.View) {
	log.Debugf("MakeProposalTimer %v", view)

	// times = times
	hs.proposeTimer = time.AfterFunc(config.Configuration.GetBigDelta(), func() {
		log.Debugf("[%v] proposal out of time, view: %v", hs.ID(), view)
		hs.processTimeOutVmo(view)
	})
}

func (hs *HotStuff) MakeVoteTimer(view types.View) {

	if hs.proposeTimer != nil {
		hs.proposeTimer.Stop()
	}
	hs.voteTimer = time.AfterFunc(config.Configuration.GetBigDelta(), func() {
		if hs.pm.GetCurView() == view {
			log.Debugf("[%v] vote out of time, view: %v", hs.ID(), view)
			hs.processTimeOutVmo(view)
		}
	})

}

func (hs *HotStuff) MakeViewChangeTimer(view types.View) {
	time.AfterFunc(config.Configuration.GetBigDelta(), func() {
		log.Debugf("[%v] view change out of time, view: %v", hs.ID(), view)

		hs.pm.AdvanceView(view)
		block := hs.getBufferedBlock(view + 1)
		hs.MakeProposalTimer(view + 1)
		if block != nil {
			hs.ProcessBlock(block)
		}
	})
}

func (hs *HotStuff) processTimeOutVmo(view types.View) {

	nextLeaderId := hs.FindLeaderFor(view + 1)
	vmo := &pacemaker.VMO{
		View:   view,
		NodeID: nextLeaderId,
		HighQC: nil,
	}
	// 超时进入view change，通知leader
	hs.MakeViewChangeTimer(view)
	vmo.NodeID = hs.ID()
	time.Sleep(config.GetConfig().GetTrueDelay())
	hs.Send(nextLeaderId, vmo)
}

func (hs *HotStuff) forkChoice() (*blockchain.QC, int, int) {
	// var choice *blockchain.QC
	if !hs.IsByz() || !config.GetConfig().ForkATK || config.GetConfig().SilentATK {
		parBlockID := hs.GetHighQC().BlockID
		parBlock, err := hs.bc.GetBlockByID(parBlockID)
		if err == nil {
			return hs.GetHighQC(), -1, parBlock.GetHeight() + 1
		} else {
			return hs.GetHighQC(), -1, hs.GetHeight() + 1
		}
	}

	return hs.forkRule()
}

func (hs *HotStuff) forkRule() (*blockchain.QC, int, int) {
	var forkNum int = 0
	var height int = 0
	var choice *blockchain.QC
	parBlockID := hs.GetHighQC().BlockID
	parBlock, err := hs.bc.GetBlockByID(parBlockID)
	grandParBlock, err1 := hs.bc.GetParentBlock(parBlockID)
	if err != nil || err1 != nil {
		forkNum = 0
		choice = hs.GetHighQC()
		height = hs.height + 1
		return choice, forkNum, height
	}

	if parBlock.View < hs.preferredView || parBlock.GetMali() {
		forkNum = 0
		choice = hs.GetHighQC()
		height = hs.height + 1
		return choice, forkNum, height
	} else {
		forkNum = 1
		choice = parBlock.QC
		height = parBlock.GetHeight()
	}
	if grandParBlock.View >= hs.preferredView && err1 == nil && !grandParBlock.GetMali() {
		forkNum = 2
		choice = grandParBlock.QC
		height = grandParBlock.GetHeight()
	}
	return choice, forkNum, height
}

func (hs *HotStuff) processTC(tc *pacemaker.TC) {
	if tc.View < hs.pm.GetCurView() {
		return
	}
	hs.pm.AdvanceView(tc.View)
}

func (hs *HotStuff) GetHighQC() *blockchain.QC {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.highQC
}

func (hs *HotStuff) GetHeight() int {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.height
}

func (hs *HotStuff) GetChainStatus() string {
	chainGrowthRate := hs.bc.GetChainGrowth()
	blockIntervals := hs.bc.GetBlockIntervals()
	return fmt.Sprintf("[%v] The current view is: %v, chain growth rate is: %v, ave block interval is: %v", hs.ID(), hs.pm.GetCurView(), chainGrowthRate, blockIntervals)
}

func (hs *HotStuff) updateHighQC(qc *blockchain.QC) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if qc.View > hs.highQC.View {
		hs.highQC = qc
	}
}

func (hs *HotStuff) processCertificate(qc *blockchain.QC) {
	log.Debugf("[%v] is processing a QC, block id: %x", hs.ID(), qc.BlockID)
	if qc.View < hs.pm.GetCurView()-2 {
		return
	}
	if qc.Leader != hs.ID() {
		quorumIsVerified, _ := crypto.VerifyQuorumSignature(qc.AggSig, qc.BlockID, qc.Signers)
		if quorumIsVerified == false {
			log.Warningf("[%v] received a quorum with invalid signatures", hs.ID())
			return
		}
	}
	err := hs.updatePreferredView(qc)
	if err != nil {
		hs.bufferedQCs[qc.BlockID] = qc
		log.Debugf("[%v] a qc is buffered, view: %v, id: %x", hs.ID(), qc.View, qc.BlockID)
		return
	}
	hs.updateHighQC(qc)
	if qc.View < 3 {
		return
	}
	ok, block, _ := hs.commitRule(qc)
	if !ok {
		return
	}
	// forked blocks are found when pruning
	committedBlocks, forkedBlocks, err := hs.bc.CommitBlock(block.ID, hs.pm.GetCurView())
	var heightestBlock *blockchain.Block

	for _, cBlock := range committedBlocks {
		hs.committedBlocks <- cBlock
		if heightestBlock == nil || int(cBlock.View) > int(heightestBlock.View) {
			heightestBlock = cBlock
		}
	}
	if heightestBlock != nil {
		heightestBlock.CommitFromThis = true
	}

	for _, fBlock := range forkedBlocks {
		hs.forkedBlocks <- fBlock
	}
}

func (hs *HotStuff) votingRule(block *blockchain.Block) (bool, error) {
	if block.View <= 3 {
		return true, nil
	}
	parentBlock, err := hs.bc.GetParentBlock(block.ID)
	if err != nil {
		return false, fmt.Errorf("cannot vote for block: %w", err)
	}
	if parentBlock.View < hs.preferredView {
		return false, nil
	}
	return true, nil
}

func (hs *HotStuff) commitRule(qc *blockchain.QC) (bool, *blockchain.Block, error) {
	parentBlock, err := hs.bc.GetParentBlock(qc.BlockID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	grandParentBlock, err := hs.bc.GetParentBlock(parentBlock.ID)
	if err != nil {
		return false, nil, fmt.Errorf("cannot commit any block: %w", err)
	}
	if ((grandParentBlock.View + 1) == parentBlock.View) && ((parentBlock.View + 1) == qc.View) {
		return true, grandParentBlock, nil
	}
	return false, nil, nil
}

func (hs *HotStuff) updatePreferredView(qc *blockchain.QC) error {
	if qc.View <= 2 {
		return nil
	}
	_, err := hs.bc.GetBlockByID(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	grandParentBlock, err := hs.bc.GetParentBlock(qc.BlockID)
	if err != nil {
		return fmt.Errorf("cannot update preferred view: %w", err)
	}
	if grandParentBlock.View > hs.preferredView {
		hs.preferredView = grandParentBlock.View
	}
	return nil
}

func (hs *HotStuff) setBufferedBlock(view types.View, block *blockchain.Block) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.bufferedBlocks[view-1] = block
}

func (hs *HotStuff) deleteBufferedBlock(view types.View) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if hs.bufferedBlocks[view] != nil {
		delete(hs.bufferedBlocks, view-1)
	}
}

func (hs *HotStuff) getBufferedBlock(view types.View) *blockchain.Block {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.bufferedBlocks[view-1]
}

func (hs *HotStuff) getRecivedBlock(view types.View) *blockchain.Block {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.recivedBlock[view-1]
}

func (hs *HotStuff) setRecivedBlock(view types.View, block *blockchain.Block) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.recivedBlock[view-1] = block
}

func (hs *HotStuff) deleteRecivedBlock(view types.View) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if hs.recivedBlock[view] != nil {
		delete(hs.recivedBlock, view-1)
	}
}
