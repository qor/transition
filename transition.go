package transition

import (
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/qor/qor/admin"
	"github.com/qor/qor/resource"
	"github.com/qor/roles"
)

type Transition struct {
	State           string
	StateChangeLogs []StateChangeLog `sql:"-"`
}

// SetState set state to Stater, just set, won't save it into database
func (transition *Transition) SetState(name string) {
	transition.State = name
}

// GetState get current state from
func (transition Transition) GetState() string {
	return transition.State
}

type Stater interface {
	SetState(name string)
	GetState() string
}

func New(value interface{}) *StateMachine {
	return &StateMachine{
		states: map[string]*State{},
		events: map[string]*Event{},
	}
}

type StateMachine struct {
	initialState string
	states       map[string]*State
	events       map[string]*Event
}

func (sm *StateMachine) Initial(name string) *StateMachine {
	sm.initialState = name
	return sm
}

func (sm *StateMachine) State(name string) *State {
	event := &State{Name: name}
	sm.states[name] = event
	return event
}

func (sm *StateMachine) Event(name string) *Event {
	event := &Event{Name: name}
	sm.events[name] = event
	return event
}

func (sm *StateMachine) Trigger(name string, value Stater, tx *gorm.DB, notes ...string) error {
	var (
		newTx    *gorm.DB
		stateWas = value.GetState()
	)

	if tx != nil {
		newTx = tx.New()
	}

	if stateWas == "" {
		stateWas = sm.initialState
		value.SetState(sm.initialState)
	}

	if event := sm.events[name]; event != nil {
		var matchedTransitions []*EventTransition
		for _, transition := range event.transitions {
			var validFrom = len(transition.froms) == 0
			if len(transition.froms) > 0 {
				for _, from := range transition.froms {
					if from == stateWas {
						validFrom = true
					}
				}
			}

			if validFrom {
				matchedTransitions = append(matchedTransitions, transition)
			}
		}

		if len(matchedTransitions) == 1 {
			transition := matchedTransitions[0]

			// State: exit
			if state, ok := sm.states[stateWas]; ok {
				for _, exit := range state.exits {
					if err := exit(value, newTx); err != nil {
						return err
					}
				}
			}

			// Transition: before
			for _, before := range transition.befores {
				if err := before(value, newTx); err != nil {
					return err
				}
			}

			value.SetState(transition.to)

			// State: enter
			if state, ok := sm.states[transition.to]; ok {
				for _, enter := range state.enters {
					if err := enter(value, newTx); err != nil {
						value.SetState(stateWas)
						return err
					}
				}
			}

			// Transition: after
			for _, after := range transition.afters {
				if err := after(value, newTx); err != nil {
					value.SetState(stateWas)
					return err
				}
			}

			if newTx != nil {
				scope := newTx.NewScope(value)
				log := StateChangeLog{
					ReferTable: scope.TableName(),
					ReferId:    GenerateReferenceKey(value, tx),
					From:       stateWas,
					To:         transition.to,
					Note:       strings.Join(notes, ""),
				}
				return newTx.Save(&log).Error
			} else {
				return nil
			}
		}
	}
	return fmt.Errorf("failed to perform event %s from state %s", name, stateWas)
}

type State struct {
	Name   string
	enters []func(value interface{}, tx *gorm.DB) error
	exits  []func(value interface{}, tx *gorm.DB) error
}

func (state *State) Enter(fc func(value interface{}, tx *gorm.DB) error) *State {
	state.enters = append(state.enters, fc)
	return state
}

func (state *State) Exit(fc func(value interface{}, tx *gorm.DB) error) *State {
	state.exits = append(state.exits, fc)
	return state
}

type Event struct {
	Name        string
	transitions []*EventTransition
}

func (event *Event) To(name string) *EventTransition {
	transition := &EventTransition{to: name}
	event.transitions = append(event.transitions, transition)
	return transition
}

type EventTransition struct {
	to      string
	froms   []string
	befores []func(value interface{}, tx *gorm.DB) error
	afters  []func(value interface{}, tx *gorm.DB) error
}

func (transition *EventTransition) From(states ...string) *EventTransition {
	transition.froms = states
	return transition
}

func (transition *EventTransition) Before(fc func(value interface{}, tx *gorm.DB) error) *EventTransition {
	transition.befores = append(transition.befores, fc)
	return transition
}

func (transition *EventTransition) After(fc func(value interface{}, tx *gorm.DB) error) *EventTransition {
	transition.afters = append(transition.afters, fc)
	return transition
}

func (transition *Transition) ConfigureQorResource(res resource.Resourcer) {
	if res, ok := res.(*admin.Resource); ok {
		if res.GetMeta("State") == nil {
			res.Meta(&admin.Meta{Name: "State", Permission: roles.Deny(roles.Update, roles.Anyone)})
		}
	}
}
