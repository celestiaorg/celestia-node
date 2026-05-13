package headers

import "sync/atomic"

type Progress struct {
	stored       atomic.Uint64
	target       atomic.Uint64
	targetHeight atomic.Uint64
}

func NewProgress() *Progress { return &Progress{} }

func (p *Progress) Init(target uint64) { p.target.Store(target) }

func (p *Progress) AddStored(n uint64) { p.stored.Add(n) }

func (p *Progress) Stored() uint64 { return p.stored.Load() }
func (p *Progress) Target() uint64 { return p.target.Load() }

func (p *Progress) SetTargetHeight(h uint64) { p.targetHeight.Store(h) }

func (p *Progress) TargetHeight() uint64 { return p.targetHeight.Load() }
