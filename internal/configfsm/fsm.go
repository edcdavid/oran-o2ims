package configfsm

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-kni/oran-o2ims/internal/controllers/utils"
	"github.com/qmuntal/stateless"
)

type Trigger string

const (
	// Transitions to Missing state
	StartToMissing           Trigger = Trigger(Start + To + Missing)
	ClusterNotReadyToMissing Trigger = Trigger(ClusterNotReady + To + Missing)

	// Transitions to ClusterNotReady state
	MissingToClusterNotReady    Trigger = Trigger(Missing + To + ClusterNotReady)
	InProgressToClusterNotReady Trigger = Trigger(InProgress + To + ClusterNotReady)

	// Transitions to InProgress state
	ClusterNotReadyToInProgress Trigger = Trigger(ClusterNotReady + To + InProgress)
	OutOfDateToInProgress       Trigger = Trigger(OutOfDate + To + InProgress)
	CompletedToInProgress       Trigger = Trigger(Completed + To + InProgress)
	TimedOutToInProgress        Trigger = Trigger(TimedOut + To + InProgress)

	// Transitions to TimedOut state
	InProgressToTimedOut Trigger = Trigger(InProgress + To + TimedOut)

	// Transitions to Completed state
	InProgressToCompleted Trigger = Trigger(InProgress + To + Completed)

	// Transitions to OutOfDate state
	InProgressToOutOfDate Trigger = Trigger(InProgress + To + OutOfDate)

	// Self transitions
	MissingToMissing                 Trigger = Trigger(Missing + To + Missing)
	ClusterNotReadyToClusterNotReady Trigger = Trigger(ClusterNotReady + To + ClusterNotReady)
	InProgressToInProgress           Trigger = Trigger(InProgress + To + InProgress)
	TimedOutToTimedOut               Trigger = Trigger(TimedOut + To + TimedOut)
	CompletedToCompleted             Trigger = Trigger(Completed + To + Completed)
	OutOfDateToOutOfDate             Trigger = Trigger(OutOfDate + To + OutOfDate)

	// States
	Start           = "Start"
	Missing         = string(utils.Missing)
	ClusterNotReady = string(utils.ClusterNotReady)
	InProgress      = string(utils.InProgress)
	TimedOut        = string(utils.TimedOut)
	Completed       = string(utils.Completed)
	OutOfDate       = string(utils.OutOfDate)
	To              = "->"
)

// RunFSM Runs the state machine as much as it can by triggering all Self transitions
func RunFSM(ctx context.Context, fsm *stateless.StateMachine, fsmHelper FsmHelper) (aState any, err error) {
	aState, err = fsm.State(ctx)
	if err != nil {
		return aState, fmt.Errorf("could not get state, err: %w", err)
	}
	switch aState {
	case Start:
		err = fsm.Fire(StartToMissing, fsmHelper)
		if err != nil {
			return aState, fmt.Errorf("could not Fire state %s, err: %w", aState, err)
		}
	case Missing:
		err = fsm.Fire(MissingToMissing, fsmHelper)
		if err != nil {
			return aState, fmt.Errorf("could not Fire state %s, err: %w", aState, err)
		}
	case ClusterNotReady:
		err = fsm.Fire(ClusterNotReadyToClusterNotReady, fsmHelper)
		if err != nil {
			return aState, fmt.Errorf("could not Fire state %s, err: %w", aState, err)
		}
	case InProgress:
		err = fsm.Fire(InProgressToInProgress, fsmHelper)
		if err != nil {
			return aState, fmt.Errorf("could not Fire state %s, err: %w", aState, err)
		}
	case TimedOut:
		err = fsm.Fire(TimedOutToTimedOut, fsmHelper)
		if err != nil {
			return aState, fmt.Errorf("could not Fire state %s, err: %w", aState, err)
		}
	case Completed:
		err = fsm.Fire(CompletedToCompleted, fsmHelper)
		if err != nil {
			return aState, fmt.Errorf("could not Fire state %s, err: %w", aState, err)
		}
	case OutOfDate:
		err = fsm.Fire(OutOfDateToOutOfDate, fsmHelper)
		if err != nil {
			return aState, fmt.Errorf("could not Fire state %s, err: %w", aState, err)
		}
	}
	// Update the current state
	aState, err = fsm.State(ctx)
	if err != nil {
		return aState, fmt.Errorf("could not get state, err: %w", err)
	}
	return aState, nil
}

// InitFSM Initialize the state machine algorithm
func InitFSM(state string) (fsm *stateless.StateMachine, err error) {
	fsm = stateless.NewStateMachine(state)
	fsm.Configure(Start).Permit(StartToMissing, Missing)
	fsm.Configure(Missing).
		OnEntry(func(_ context.Context, args ...any) error {
			fsmHelper := args[0].(FsmHelper)
			fmt.Println("Entering " + Missing)
			fsmHelper.ResetNonCompliantAt()
			if fsmHelper.ArePoliciesMatched() {
				return fsm.Fire(MissingToClusterNotReady, fsmHelper)
			}
			return nil
		}).
		Permit(MissingToClusterNotReady, ClusterNotReady).
		PermitReentry(MissingToMissing)
	fsm.Configure(ClusterNotReady).
		OnEntry(func(_ context.Context, args ...any) error {
			fsmHelper := args[0].(FsmHelper)
			fmt.Println("Entering " + ClusterNotReady)
			if !fsmHelper.ArePoliciesMatched() {
				return fsm.Fire(ClusterNotReadyToMissing, fsmHelper)
			}
			if fsmHelper.IsClusterReady() ||
				fsmHelper.IsTimedOut() {
				return fsm.Fire(ClusterNotReadyToInProgress, fsmHelper)
			}
			return nil
		}).
		Permit(ClusterNotReadyToMissing, Missing).
		Permit(ClusterNotReadyToInProgress, InProgress).
		PermitReentry(ClusterNotReadyToClusterNotReady)
	fsm.Configure(InProgress).
		OnEntry(func(_ context.Context, args ...any) error {
			fsmHelper := args[0].(FsmHelper)
			fmt.Println("Entering " + InProgress)
			if fsmHelper.IsNonCompliantAtZero() {
				fsmHelper.SetNonCompliantAtNow()
			}
			if fsmHelper.AreAllPoliciesCompliant() ||
				(!fsmHelper.IsNonCompliantPolicyInEnforce() && !fsmHelper.AreAllPoliciesCompliant()) {
				fsmHelper.ResetNonCompliantAt()
			}
			if !fsmHelper.ArePoliciesMatched() {
				return fsm.Fire(InProgressToClusterNotReady, fsmHelper)
			}
			if fsmHelper.IsTimedOut() {
				return fsm.Fire(InProgressToTimedOut, fsmHelper)
			}
			if !fsmHelper.IsClusterReady() {
				return fsm.Fire(InProgressToClusterNotReady, fsmHelper)
			}
			if fsmHelper.ArePoliciesOutOfDate() {
				return fsm.Fire(InProgressToOutOfDate, fsmHelper)
			}
			if fsmHelper.AreAllPoliciesCompliant() {
				return fsm.Fire(InProgressToCompleted, fsmHelper)
			}
			return nil
		}).
		Permit(InProgressToClusterNotReady, ClusterNotReady).
		Permit(InProgressToTimedOut, TimedOut).
		Permit(InProgressToOutOfDate, OutOfDate).
		Permit(InProgressToCompleted, Completed).
		PermitReentry(InProgressToInProgress)
	fsm.Configure(OutOfDate).
		OnEntry(func(_ context.Context, args ...any) error {
			fsmHelper := args[0].(FsmHelper)
			fmt.Println("Entering " + OutOfDate)
			fsmHelper.ResetNonCompliantAt()
			if !fsmHelper.ArePoliciesMatched() ||
				!fsmHelper.IsClusterReady() ||
				fsmHelper.IsNonCompliantPolicyInEnforce() ||
				fsmHelper.AreAllPoliciesCompliant() {
				return fsm.Fire(OutOfDateToInProgress, fsmHelper)
			}
			return nil
		}).
		Permit(OutOfDateToInProgress, InProgress).
		PermitReentry(OutOfDateToOutOfDate)
	fsm.Configure(Completed).
		OnEntry(func(_ context.Context, args ...any) error {
			fsmHelper := args[0].(FsmHelper)
			fmt.Println("Entering " + Completed)
			fsmHelper.ResetNonCompliantAt()

			if !fsmHelper.ArePoliciesMatched() ||
				!fsmHelper.IsClusterReady() ||
				!fsmHelper.AreAllPoliciesCompliant() {
				return fsm.Fire(CompletedToInProgress, fsmHelper)
			}
			return nil
		}).
		Permit(CompletedToInProgress, InProgress).
		PermitReentry(CompletedToCompleted)
	fsm.Configure(TimedOut).
		OnEntry(func(_ context.Context, args ...any) error {
			fsmHelper := args[0].(FsmHelper)
			fmt.Println("Entering " + TimedOut)

			if !fsmHelper.ArePoliciesMatched() ||
				fsmHelper.ArePoliciesOutOfDate() ||
				fsmHelper.AreAllPoliciesCompliant() {
				return fsm.Fire(TimedOutToInProgress, fsmHelper)
			}
			return nil
		}).
		Permit(TimedOutToInProgress, InProgress).
		PermitReentry(TimedOutToTimedOut)

	return fsm, nil
}

// FsmHelper Helper Interface to collect all functions needed for state machine decisions
type FsmHelper interface {
	ArePoliciesMatched() bool
	ResetNonCompliantAt()
	IsNonCompliantAtZero() bool
	SetNonCompliantAtNow()
	IsTimedOut() bool
	IsClusterReady() bool
	AreAllPoliciesCompliant() bool
	ArePoliciesOutOfDate() bool
	IsNonCompliantPolicyInEnforce() bool
}

// BaseFSMHelper Helper struct to store information needed for state machine decisions
type BaseFSMHelper struct {
	CurrentState                string
	ClusterReady                bool
	PoliciesMatched             bool
	AllPoliciesCompliant        bool
	NonCompliantAt              time.Time
	NonCompliantPolicyInEnforce bool
	ConfigTimeout               int
}

// ArePoliciesMatched Returns true if there are policies matched to the managed cluster, false otherwise
func (h *BaseFSMHelper) ArePoliciesMatched() bool {
	return h.PoliciesMatched
}

// ResetNonCompliantAt Resets the NonCompliantAt field to zero
func (h *BaseFSMHelper) ResetNonCompliantAt() {
	h.NonCompliantAt = time.Time{}
}

// IsNonCompliantAtZero Returns true if NonCompliantAt is was reset to zero, false otherwise
func (h *BaseFSMHelper) IsNonCompliantAtZero() bool {
	return h.NonCompliantAt == time.Time{}
}

// SetNonCompliantAtNow Sets the NonCompliantAt to the current time
func (h *BaseFSMHelper) SetNonCompliantAtNow() {
	println("Set NonCompliantAt to now")
	h.NonCompliantAt = time.Now()
}

// IsClusterReady Returns true if the Managed cluster is ready for policy enforcement, false otherwise
func (h *BaseFSMHelper) IsClusterReady() bool {
	return h.ClusterReady
}

// AreAllPoliciesCompliant Returns true if all policies are enforced and compliant, false otherwise
func (h *BaseFSMHelper) AreAllPoliciesCompliant() bool {
	return h.AllPoliciesCompliant
}

// ArePoliciesOutOfDate Returns true if there are some non compliant policies that are not
// in enforce, false otherwise
func (h *BaseFSMHelper) ArePoliciesOutOfDate() bool {
	return !h.AllPoliciesCompliant && !h.NonCompliantPolicyInEnforce
}

// IsNonCompliantPolicyInEnforce Returns true if there is at least one non compliant policy in enforce, false otherwise
func (h *BaseFSMHelper) IsNonCompliantPolicyInEnforce() bool {
	return h.NonCompliantPolicyInEnforce
}
