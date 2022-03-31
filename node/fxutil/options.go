package fxutil

import (
	"fmt"
	"reflect"

	"go.uber.org/fx"
)

// FIXME: This file is intended to be removed once the upstream issue https://github.com/uber-go/fx/issues/825
//  is resolved

// Option mimics fx.Option but provides one more OverrideSupply feature.
type Option func(*fxOptions) error

// ParseOptions parses multiple given instances of Option and coverts them into a fx.Option.
func ParseOptions(opts ...Option) (fx.Option, error) {
	fopts := &fxOptions{
		overrideSupplies: map[reflect.Type]*override{},
		provides:         map[reflect.Type]*provide{},
		supplies:         map[reflect.Type]interface{}{},
	}
	for _, opt := range opts {
		err := opt(fopts)
		if err != nil {
			return nil, err
		}
	}

	parsed := make([]fx.Option, 0)
out:
	for tp, val := range fopts.provides {
		allAs := make([]fx.Annotation, len(val.as))
		for i, as := range val.as {
			ovr, ok := fopts.overrideSupplies[as]
			if ok && !ovr.used {
				ovr.used = true
				continue out
			}

			allAs[i] = fx.As(reflect.New(as).Interface())
		}

		if len(allAs) == 0 {
			// exclude provides that are overridden
			// this has a potentially negative effect for provides with more than one return parameters
			for i := 0; i < tp.NumOut(); i++ {
				ovr, ok := fopts.overrideSupplies[tp.Out(i)]
				if ok && !ovr.used {
					ovr.used = true
					continue out
				}
			}
		}

		parsed = append(parsed, fx.Provide(fx.Annotate(val.target, allAs...)))
	}

	for tp, val := range fopts.supplies {
		ovr, ok := fopts.overrideSupplies[tp]
		if ok && !ovr.used {
			ovr.used = true
			continue
		}

		switch tp.Kind() {
		case reflect.Interface:
			// if type is an interface - we have to say FX to build the `val` as the interface
			parsed = append(parsed, fx.Supply(fx.Annotate(
				val,
				fx.As(reflect.New(tp).Interface()),
			)))
		default:
			parsed = append(parsed, fx.Supply(val))
		}
	}

	for _, val := range fopts.invokes {
		parsed = append(parsed, fx.Invoke(val))
	}

	for tp, val := range fopts.overrideSupplies {
		if !val.used {
			return nil, fmt.Errorf("fxutil: nothing to override for the given %s", tp)
		}

		switch tp.Kind() {
		case reflect.Interface:
			// if type is an interface - we have to say FX to build the `val` as the interface
			parsed = append(parsed, fx.Supply(fx.Annotate(
				val.target,
				fx.As(reflect.New(tp).Interface()),
			)))
		default:
			parsed = append(parsed, fx.Supply(val.target))
		}
	}

	return fx.Options(parsed...), nil
}

// OverrideSupply overwrites values passed to Provide and Supply.
// It mimics fx.Supply with one difference - it does not error if there are multiple instances of a type
// are supplied/provided. Instead, OverrideSupply forces the supply and overrides other Supply and Provide.
//
// Only one overriding must exist for a type.
//
// Only recvs ptr to a value or an interface.
// To override an interface a ptr to the interface should be provided:
//
//  var r io.Reader
//  OverrideSupply(&r)
//
// and not:
//
//  var r io.Reader
//  OverrideSupply(r)
//
// Otherwise, real value of r will be provided, e.g. *bytes.Buffer.
func OverrideSupply(vals ...interface{}) Option {
	return func(o *fxOptions) error {
		for _, val := range vals {
			refVal := reflect.ValueOf(val)
			if refVal.Kind() != reflect.Ptr {
				return fmt.Errorf("fxutil: only ptrs are allowed in OverrideSupply")
			}

			tp := refVal.Type()
			if tp.Elem().Kind() == reflect.Interface {
				// if ptr to an interface is passed - unwrap the interface type and a real value
				refVal, tp = refVal.Elem().Elem(), tp.Elem()
			} else if tp.Elem().Kind() != reflect.Struct {
				// if prt is not pointing at struct, e.g. Slice/String/... - unwrap the value
				refVal, tp = refVal.Elem(), tp.Elem()
			}

			if !refVal.IsValid() || refVal.IsZero() {
				// All the really invalid cases are checked and errors for them are thrown above the line.
				// Regarding this case, it is basically nil or zero value, and it is usually fine to allow zeros to be
				// passed in the optional pattern, e.g WithSomething(nil), so you don't need to check for
				// if something != nil { WithSomething(something) }.
				continue
			}

			_, ok := o.overrideSupplies[tp]
			if ok {
				return fmt.Errorf("fxutil: already overridden")
			}
			o.overrideSupplies[tp] = &override{target: refVal.Interface()}
		}

		return nil
	}
}

// Supply mimics fx.Supply.
func Supply(vals ...interface{}) Option {
	return func(o *fxOptions) error {
		for _, val := range vals {
			tp := reflect.TypeOf(val)
			_, ok := o.supplies[tp]
			if ok {
				return fmt.Errorf("fxutil: already supplied")
			}

			o.supplies[tp] = val
		}

		return nil
	}
}

// SupplyAs mimics fx.Supply(fx.Annotate(fx.As())).
func SupplyAs(val interface{}, as interface{}) Option {
	return func(o *fxOptions) error {
		tp := reflect.TypeOf(as)
		if tp.Kind() != reflect.Ptr && tp.Elem().Kind() != reflect.Interface {
			return fmt.Errorf("fxutil: As values must be ptr to an interface")
		}
		tp = tp.Elem()

		_, ok := o.supplies[tp]
		if ok {
			return fmt.Errorf("fxutil: already supplied")
		}

		o.supplies[tp] = val
		return nil
	}
}

// Provide mimics fx.Provide.
// Provided ctors can be overridden by OverrideSupply.
func Provide(vals ...interface{}) Option {
	opts := make([]Option, len(vals))
	for i, val := range vals {
		opts[i] = ProvideAs(val)
	}
	return Options(opts...)
}

// ProvideAs mimics fx.Provide(fx.Annotate(fx.As())).
func ProvideAs(val interface{}, as ...interface{}) Option {
	return func(o *fxOptions) error {
		tp := reflect.TypeOf(val)
		if tp.Kind() != reflect.Func || tp.NumOut() == 0 {
			return fmt.Errorf("fxutil: ctor functions must be passed to Provide and have return params")
		}

		defAs := make([]reflect.Type, len(as))
		for i, as := range as {
			tp := reflect.TypeOf(as)
			if tp.Kind() != reflect.Ptr && tp.Elem().Kind() != reflect.Interface {
				return fmt.Errorf("fxutil: As values must be ptr to an interface")
			}

			defAs[i] = tp.Elem()
		}

		_, ok := o.provides[tp]
		if ok {
			return fmt.Errorf("fxutil: already provided")
		}

		o.provides[tp] = &provide{
			target: val,
			as:     defAs,
		}
		return nil
	}
}

// Invoke mimics fx.Invoke.
func Invoke(vals ...interface{}) Option {
	return func(o *fxOptions) error {
		o.invokes = append(o.invokes, vals...)
		return nil
	}
}

// Options combines multiples options into a one Option.
func Options(opts ...Option) Option {
	return func(o *fxOptions) error {
		for _, opt := range opts {
			err := opt(o)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

type fxOptions struct {
	overrideSupplies map[reflect.Type]*override
	provides         map[reflect.Type]*provide
	supplies         map[reflect.Type]interface{}
	invokes          []interface{}
}

type override struct {
	target interface{}
	used   bool
}

type provide struct {
	target interface{}
	as     []reflect.Type
}
