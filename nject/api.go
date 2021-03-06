package nject

// TODO: is CallsInner() useful?  Remove if not.

import (
	"fmt"
	"reflect"
)

// Collection holds a sequence of providers and sub-collections
type Collection struct {
	name     string
	contents []*provider
}

// Provider is an individual injector (function, constant, or
// wrapper).  Functions that take injectors, take interface{}.
// Functions that return invjectors return Provider so that
// methods can be attached.
type Provider interface {
	thing
}

// Sequence creates a Collection of providers.  Each collection must
// have a name.  The providers can be: functions, variables, or
// *Collections, or *Providers.
//
// Functions must match one of the expected patterns.
//
// Injectors specified here will be separated into two sets:
// ones that are run once per bound chain (STATIC); and
// ones that are run for each invocation (RUN).  Memoized
// functions are in the STATIC chain but they only get run once
// per input combination.  Literal values are inserted before
// the STATIC chain starts.
//
// Each set will run in the order they were given here.  Providers
// whose output is not consumed are skipped unless they are marked
// with Required.  Providers that produce no output are always run.
//
// Previsously created *Collection objects are considered providers along
// with *Provider, named functions, anoymous functions, and literal values.
func Sequence(name string, providers ...interface{}) *Collection {
	return newCollection(name, providers...)
}

// Append appends additional providers onto an existing collection
// to create a new collection.  The additional providers may be
// value literals, functions, Providers, or *Collections.
func (c *Collection) Append(name string, funcs ...interface{}) *Collection {
	nc := newCollection(name, funcs...)
	nc.contents = append(c.contents, nc.contents...)
	return nc
}

// Provide wraps an individual provider.  It allows the provider
// to be named.  The return value is chainable with with annotations
// like Cacheable() and Required().  It can be included in a collection.
// When providers are not named, they get their name from their position
// in their collection combined with the name of the collection they are in.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Provide(name string, fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.origin = name
	})
}

// MustCache creates an Inject item and annotates it as required to be
// in the STATIC set.  If it cannot be placed in the STATIC set
// then any collection that includes it is invalid.
func MustCache(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.mustCache = true
		fm.cacheable = true
	})
}

// Cacheable creates an inject item and annotates it as allowed to be
// in the STATIC chain.  Without this annotation, MustCache, or
// Memoize, a provider will be in the RUN chain.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Cacheable(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.cacheable = true
	})
}

// NotCacheable creates an inject item and annotates it as not allowed to be
// in the STATIC chain.  With this annotation, Cacheable is ignored and MustCache
// causes an invalid chain.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func NotCacheable(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.notCacheable = true
	})
}

// Memoize creates a new InjectItem that is tagged as Cacheable
// further annotated so that it only executes once per
// input parameter values combination.  This cache is global among
// all Sequences.  Memoize can only be used on functions
// whose inputs are valid map keys (interfaces, arrays
// (not slices), structs, pointers, and primitive types).
//
// Memoize is further restricted in that it only works in the
// STATIC provider set.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Memoize(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.memoize = true
		fm.cacheable = true
	})
}

// Required creates a new provider and annotates it as
// required: it will be included in the provider chain even
// if its outputs are not used.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Required(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.required = true
	})
}

// Desired creates a new provider and annotates it as
// desired: it will be included in the provider chain
// unless doing so creates an un-met dependency.
//
// Injectors and wrappers that have no outputs are automatically
// considered desired.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Desired(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.desired = true
	})
}

// MustConsume creates a new provider and annotates it as
// needing to have all of its output values consumed.  If
// any of its output values cannot be consumed then the
// provider will be excluded from the chain even if that
// renders the chain invalid.
//
// A that is received by a provider and then provided by
// that same provider is not considered to have been consumed
// by that provider.
//
// For example:
//
//	// All outputs of A must be consumed
//	Provide("A", MustConsume(func() string) { return "" } ),
//
//	// Since B takes a string and provides a string it
//	// does not count as suming the string that A provided.
//	Provide("B", func(string) string { return "" }),
//
//	// Since C takes a string but does not provide one, it
//	// counts as consuming the string that A provided.
//	Provide("C", func(string) int { return 0 }),
//
// MustConsume works only in the downward direction of the
// provider chain.  In the upward direction (return values)
// all values must be consumed.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func MustConsume(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.mustConsume = true
	})
}

// ConsumptionOptional creates a new provider and annotates it as
// allowed to have some of its return values ignored.
// Without this annotation, a wrap function will not be included
// if some of its return values are not consumed.
//
// In the downward direction, optional consumption is the default.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func ConsumptionOptional(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.consumptionOptional = true
	})
}

// Is this useful?   Don't export for now.
//
// CallsInner annotates a wrap function to promise that it
// will always invoke its inner() function.  Without this
// annotation, wrap functions that come before other wrap
// functions or the final function that return values that
// do not have a zero value that can be created with reflect
// are invalid.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func callsInner(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.callsInner = true
	})
}

// Loose annotates a wrap function to indicate that when trying
// to match types against the outputs and return values from this
// provider, an in-exact match is acceptable.  This matters when inputs and
// returned values are specified as interfaces.  With the Loose
// annotation, an interface can be matched to the outputs and/or
// return values of this provider if the output/return value
// implements the interface.
//
// By default, an exact match of types is required for all providers.
func Loose(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.loose = true
	})
}

// Bind expects to receive two function pointers for functions
// that are not yet defined.  Bind defines the functions.  The
// first function is called to invoke the Collection of providers.
//
// The inputs to the invoke function are passed into the provider
// chain.  The value returned from the invoke function comes from
// the values returned by the provider chain (from middleware and
// from the final func).
//
// The second function is optional.  It is called to initialize the
// provider chain.  Once initialized, any further calls to the initialize
// function are ignored.
//
// The inputs to the initialization function are injected into the
// head of the provider chain.  The static portion of the provider
// chain will run once.  The values returned from the initialization
// function come from the values available after the static portion
// of the provider chain runs.
//
// Bind pre-computes as much as possible so that the invokeFunc is
// fast.
func (c *Collection) Bind(invokeFunc interface{}, initFunc interface{}) error {
	if err := c.bindFast(invokeFunc, initFunc); err != nil {
		invokeF := newProvider(invokeFunc, -1, c.name+" invoke func")
		var initF *provider
		if initFunc != nil {
			initF = newProvider(initFunc, -1, c.name+" initialization func")
		}

		debugOutput := captureDoBindDebugging(c, invokeF, initF)
		return &njectError{
			err:     err,
			details: debugOutput,
		}
	}
	return nil
}

func (c *Collection) bindFast(invokeFunc interface{}, initFunc interface{}) error {
	invokeF := newProvider(invokeFunc, -1, c.name+" invoke func")
	var initF *provider
	if initFunc != nil {
		initF = newProvider(initFunc, -1, c.name+" initialization func")
	}

	debugLock.RLock()
	defer debugLock.RUnlock()
	return doBind(c, invokeF, initF, true)
}

// TODO: add an example

// SetCallback expects to receive a function as an argument.  SetCallback() will call
// that function.  That function in turn should take one or two functions
// as arguments.  The first argument must be an invoke function (see Bind).
// The second argument (if present) must be an init function.  The invoke func
// (and the init func if present) will be created by SetCallback() and passed
// to the function SetCallback calls.
func (c *Collection) SetCallback(setCallbackFunc interface{}) error {
	setter := reflect.ValueOf(setCallbackFunc)
	setterType := setter.Type()
	if setterType.Kind() != reflect.Func {
		return fmt.Errorf("SetCallback must be passed a function")
	}
	if setterType.NumIn() < 1 || setterType.NumIn() > 2 {
		return fmt.Errorf("SetCallback function argument must take one or two inputs")
	}
	if setterType.NumOut() > 0 {
		return fmt.Errorf("SetCallback function argument must return nothing")
	}
	for i := 0; i < setterType.NumIn(); i++ {
		if setterType.In(i).Kind() != reflect.Func {
			return fmt.Errorf("SetCallback function argument #%d must be a function", i+1)
		}
	}
	var err error
	invokePtr := reflect.New(setterType.In(0)).Interface()
	if setterType.NumIn() == 2 {
		initPtr := reflect.New(setterType.In(1)).Interface()
		err = c.Bind(invokePtr, initPtr)
		if err == nil {
			setter.Call([]reflect.Value{
				reflect.ValueOf(invokePtr).Elem(),
				reflect.ValueOf(initPtr).Elem()})
		}
	} else {
		err = c.Bind(invokePtr, nil)
		if err == nil {
			setter.Call([]reflect.Value{reflect.ValueOf(invokePtr).Elem()})
		}
	}
	return err
}

// Run is different from bind: the provider chain is run, not bound
// to functions.
//
// The only return value from the final function that is captured by Run()
// is error.  Run will return that error value.  If the final function does
// not return error, then run will return nil if it was able to execute the
// collection and function.  Run can return error because the final function
// returned error or because the provider chain was not valid.
//
// Nothing is pre-computed with Run(): the run-time cost from nject is higher
// than calling an invoke function defined by Bind().
//
// Predefined Collection objects are considered providers along with InjectItems,
// functions, and literal values.
func Run(name string, providers ...interface{}) error {
	c := Sequence(name,
		// include a default error responder so that the
		// error return from Run() does not pull in any
		// providers.
		Provide("Run()error", func() TerminalError {
			return nil
		})).
		Append(name, providers...)
	var invoke func() error
	err := c.Bind(&invoke, nil)
	if err != nil {
		return err
	}
	return invoke()
}

// MustRun is a wrapper for Run().  It panic()s if Run() returns error.
func MustRun(name string, providers ...interface{}) {
	err := Run(name, providers...)
	if err != nil {
		panic(err)
	}
}

// MustBindSimple binds a collection with an invoke function that takes no
// arguments and returns no arguments.  It panic()s if Bind() returns error.
func MustBindSimple(c *Collection, name string) func() {
	var invoke func()
	MustBind(c, &invoke, nil)
	return invoke
}

// MustBindSimpleError binds a collection with an invoke function that takes no
// arguments and returns error.
func MustBindSimpleError(c *Collection, name string) func() error {
	var invoke func() error
	MustBind(c, &invoke, nil)
	return invoke
}

// MustBind is a wrapper for Collection.Bind().  It panic()s if Bind() returns error.
func MustBind(c *Collection, invokeFunc interface{}, initFunc interface{}) {
	err := c.Bind(invokeFunc, initFunc)
	if err != nil {
		panic(DetailedError(err))
	}
}

// MustSetCallback is a wrapper for Collection.SetCallback().  It panic()s if SetCallback() returns error.
func MustSetCallback(c *Collection, binderFunction interface{}) {
	err := c.SetCallback(binderFunction)
	if err != nil {
		panic(DetailedError(err))
	}
}
