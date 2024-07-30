package election

import (
	"crypto/sha1"
	"encoding/binary"
	"strconv"

	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/types"
)

type Rotation struct {
	peerNo int
	seed   int64
}

func NewRotation(peerNo int) *Rotation {
	// 生成一个随机数
	seed := config.GetConfig().Seed

	return &Rotation{
		peerNo: peerNo,
		seed:   seed,
	}
}

func (r *Rotation) IsLeader(id identity.NodeID, view types.View) bool {
	if view <= 3 {
		if id.Node() < r.peerNo {
			return false
		}
		return true
	}
	h := sha1.New()
	h.Write([]byte(strconv.Itoa(int(view) + 1)))
	bs := h.Sum(nil)
	data := binary.BigEndian.Uint64(bs) + uint64(r.seed)
	return data%uint64(r.peerNo) == uint64(id.Node()-1)
}

func (r *Rotation) FindLeaderFor(view types.View) identity.NodeID {
	if view <= 3 {
		return identity.NewNodeID(r.peerNo)
	}
	h := sha1.New()
	h.Write([]byte(strconv.Itoa(int(view + 1))))
	bs := h.Sum(nil)
	data := binary.BigEndian.Uint64(bs) + uint64(r.seed)
	id := data%uint64(r.peerNo) + 1
	return identity.NewNodeID(int(id))
}
