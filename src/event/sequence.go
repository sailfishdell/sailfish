package eventsourcing

type Sequencer interface {
	GetSequence() int
	SetSequence(int)
}

type withSequence struct {
	Seq int
}

func (s *withSequence) GetSequence() int {
	return s.Seq
}

func (s *withSequence) SetSequence(ns int) {
	s.Seq = ns
}
