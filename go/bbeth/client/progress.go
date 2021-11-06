package client

import (
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

// TransactionProgress operates progress meters for issuing and observing
// tranaactions. There is one meter for issuing and one for tracking mined
// transactions. Each can be independently enabled. If both are disabled all
// methods are safe to call and have no effect.
type TransactionProgress struct {
	numExpected int
	pb          *mpb.Progress
	pbTxIssued  *mpb.Bar
	pbTxMined   *mpb.Bar
	numMined    int // needed so UpdateMined can work when progress bars are both disabled
}

type ProgressOption func(*TransactionProgress)

// WithIssuedProgress enables the progress bar for issuing transactions
func WithIssuedProgress() ProgressOption {
	// Note options must be applied *after* the cfg is saved in the new
	// instance
	return func(p *TransactionProgress) {
		if p.pb == nil {
			p.pb = mpb.New()
		}
		numExpected := p.numExpected
		if numExpected == -1 {
			numExpected = p.numExpected
		}
		p.pbTxIssued = p.pb.AddBar(
			int64(numExpected), mpb.PrependDecorators(
				decor.Name("sent", decor.WCSyncSpace),
				decor.CurrentNoUnit("%d", decor.WCSyncSpace),
				decor.AverageSpeed(0, "%f.2/s", decor.WCSyncSpace),
				decor.Elapsed(decor.ET_STYLE_MMSS, decor.WCSyncSpace),
			),
		)
	}
}

// WithMinedProgress enables the progress bar for issuing transactions
func WithMinedProgress() ProgressOption {
	// Note options must be applied *after* the cfg is saved in the new
	// instance
	return func(p *TransactionProgress) {
		if p.pb == nil {
			p.pb = mpb.New()
		}
		numExpected := p.numExpected
		if numExpected == -1 {
			numExpected = 0
		}
		p.pbTxMined = p.pb.AddBar(
			int64(numExpected),
			mpb.PrependDecorators(
				decor.Name("mined", decor.WCSyncSpace),
				decor.CurrentNoUnit("%d", decor.WCSyncSpace),
				decor.AverageSpeed(0, "%f.2/s", decor.WCSyncSpace),
				decor.Elapsed(decor.ET_STYLE_MMSS, decor.WCSyncSpace),
			),
		)
	}
}

func NewTransactionProgress(numExpected int, opts ...ProgressOption) *TransactionProgress {
	tp := &TransactionProgress{numExpected: numExpected}
	for _, opt := range opts {
		opt(tp)
	}
	return tp
}

func (p *TransactionProgress) NumMined() int {
	return p.numMined
}

// MinedComplete updates the progress bar and completes if if we have mined at
// least the target number of transactions. It returns true if we have reached
// that target. This method tracks the number of transactions independently of
// the progress bars, so it always returns an acurate indication of completion.
func (p *TransactionProgress) MinedComplete(ntx int) bool {

	p.numMined += ntx

	p.MinedIncrBy(ntx)

	if p.numMined < p.numExpected || p.numExpected == -1 {
		return false
	}
	p.SetTotalMined(int64(p.numExpected), true)
	return true
}

func (p *TransactionProgress) IsEnabled() bool {
	return p.pb != nil
}

func (p *TransactionProgress) IssuedIncrement() {
	if p.pbTxIssued == nil {
		return
	}
	p.pbTxIssued.Increment()
}

func (p *TransactionProgress) IssuedIncrBy(n int) {
	if p.pbTxIssued == nil {
		return
	}
	p.pbTxIssued.IncrBy(n)
}

func (p *TransactionProgress) MinedIncrement() {
	if p.pbTxMined == nil {
		return
	}
	p.pbTxMined.Increment()
}

func (p *TransactionProgress) MinedIncrBy(n int) {
	if p.pbTxMined == nil {
		return
	}
	p.pbTxMined.IncrBy(n)
}

func (p *TransactionProgress) CurrentIssued() int64 {
	if p.pbTxIssued == nil {
		return -1
	}
	return p.pbTxIssued.Current()
}

func (p *TransactionProgress) CurrentMined() int64 {
	if p.pbTxMined == nil {
		return -1
	}
	return p.pbTxMined.Current()
}

func (p *TransactionProgress) SetTotalIssued(total int64, complete bool) {
	if p.pbTxIssued == nil {
		return
	}
	p.pbTxIssued.SetTotal(total, complete)
}

func (p *TransactionProgress) SetTotalMined(total int64, complete bool) {
	if p.pbTxMined == nil {
		return
	}
	p.pbTxMined.SetTotal(total, complete)
}
