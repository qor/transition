# Transition

Transition is a Golang state machine implementation.

Transition could be used standalone, but works better with [GORM](https://github.com/jinzhu/gorm) models, it could keep state change logs into database automatically.

# Usage

### Using Transition

Add Transition to your struct, it will add some state machine related methods to the struct

```go
import "github.com/qor/transition"

type Order struct {
  ID uint
  transition.Transition
}

var order Order

// Get Current State
order.GetState()

// Set State
order.SetState("finished")
```

### Define States and Events

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

// Define more States
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
// func (*StateMachine) Trigger(name string, value Stater, tx *gorm.DB, notes ...string) error
OrderStatemachine.Trigger("paid", &order, db, "charged offline by jinzhu")
// notes will be used to generate state change logs when works with GORM

// When use without GORM, just pass nil to the db, like
OrderStatemachine.Trigger("cancel", &order, nil)

OrderStatemachine.Trigger("cancel", &order, db)
// order's state will be changed to cancelled if current state is "draft"
// order's state will be changed to paid_cancelled if current state is "paid"
```

## State change logs

When works with GORM, it will keep all state change logs into database, use GetStateChangeLogs to get those logs

```go
var stateChangeLogs = transition.GetStateChangeLogs(&order, db)

// type StateChangeLog struct {
// 	 From       string  // from state
// 	 To         string  // to state
// 	 Note       string  // notes
// }
```

## License

Released under the [MIT License](http://opensource.org/licenses/MIT).
