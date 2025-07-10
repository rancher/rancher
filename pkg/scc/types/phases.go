package types

type Phase interface {
	String() string
	GroupName() string
	IndividualName() string
}

type RegistrationPhase int

const (
	// RegistrationInit is unused (for now) but represents any phase before things actually get started
	RegistrationInit RegistrationPhase = iota
	// RegistrationPrepare happens before registration/announce of the cluster
	RegistrationPrepare
	// RegistrationMain is the core registration process
	RegistrationMain
	// RegistrationForActivation takes a successful RegMain and prepares for activation
	// critically this is part of Reg, because it happens after successful Reg outside of Activation process
	RegistrationForActivation
)

func (r RegistrationPhase) String() string {
	return r.IndividualName()
}

func (r RegistrationPhase) GroupName() string {
	return "Registration"
}

func (r RegistrationPhase) IndividualName() string {
	return [...]string{"Init", "PrepareForReg", "Main", "PrepForActivation"}[r]
}

type ActivationPhase int

const (
	// ActivationInit is unused (for now) but represents any phase before things actually get started
	ActivationInit ActivationPhase = iota
	ActivationMain
	ActivationPrepForKeepalive
)

func (a ActivationPhase) String() string {
	return a.IndividualName()
}

func (a ActivationPhase) GroupName() string {
	return "Activation"
}

func (a ActivationPhase) IndividualName() string {
	return [...]string{"Init", "Main"}[a]
}

var _ Phase = RegistrationPhase(0)
var _ Phase = ActivationPhase(0)
