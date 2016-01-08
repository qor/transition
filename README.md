# Transition

Transition is a Golang state machine implementation. rely on [GORM](github.com/jinzhu/gorm)

# Installation

```
go get github.com/qor/transition
```

# Usage

### Include transition to model, it will add State, StateChangeLogs to the model

```go
type Order struct {
  gorm.Model
  ...

  // Add transition to Order
  transition.Transition
  // type Transition struct {
  //   State           string
  //   StateChangeLogs []StateChangeLog `sql:"-"`
  // }
}
```

### Define states and events for a model

```go
var OrderStateMachine = transition.New(&Order{})

// Define initial state
OrderStateMachine.Initial("draft")

// Define a State
OrderStateMachine.State("checkout")

// Define another State and what to do when enter a state and exit a state.
OrderStateMachine.State("paid").Enter(func(order interface{}, tx *gorm.DB) error {
  // To get order object use 'order.(*Order)'
  // business logic here
  return
}).Exit(func(order interface{}, tx *gorm.DB) error {
  // business logic here
  return
})

OrderStateMachine.State("cancelled")
OrderStateMachine.State("paid_cancelled")

// Define an Event
OrderStateMachine.Event("checkout").To("checkout").From("draft")

// Define another event and what to do before perform transition and after transition.
OrderStateMachine.Event("paid").To("paid").From("checkout").Before(func(order interface{}, tx *gorm.DB) error {
  // business logic here
  return
}).After(func(order interface{}, tx *gorm.DB) error {
  // business logic here
  return
})

// Different state transitions for one event
cancellEvent := OrderStateMachine.Event("cancel")
cancellEvent.To("cancelled").From("draft", "checkout")
cancellEvent.To("paid_cancelled").From("paid").After(func(order interface{}, tx *gorm.DB) error {
  // Refund
}})
```

### Trigger an Event

```go
// func (sm *StateMachine) Trigger(name string, value Stater, tx *gorm.DB, notes ...string) error
// notes will be used to generate state change logs
OrderStatemachine.Trigger("paid", &order, db, "charged offline by jinzhu")

OrderStatemachine.Trigger("cancel", &order, db)
// order's state will be changed to cancelled if current state is "draft"
// order's state will be changed to paid_cancelled if current state is "paid"
```

## State change logs

```go
// For each state change, transition will auto create a change log for this
// use GetStateChangeLogs to get those logs
transition.GetStateChangeLogs(&order, db)
```

## License

Released under the [MIT License](http://opensource.org/licenses/MIT).
