# goraygen

`goraygen` is a CLI tool that generates type-safe wrapper functions for remote calls in [GoRay](https://github.com/ray4go/go-ray#) applications.

## Features

The `goraygen`-generated wrappers provides:

- **Compile-time type checking**: Unlike the original `ray.RemoteCall()` and `ray.NewActor()`, which use string-based task/actor names and `any`-typed parameters, generated wrappers enable full compile-time verification of parameter types
- **Type-safe return values**: Future objects include explicit type information, eliminating the need for manual type assertions
- **Enhanced developer experience**: Wrapper APIs bring full IDE integration to Ray task and actor invocations, with less boilerplate code.

## Installation

```bash
go install "github.com/ray4go/goraygen@latest"
```

<!-- If you encounter Go version compatibility errors, install a version matching your Go installation:

```bash
GO_VER=$(go env GOVERSION | sed 's/go//' | cut -d'.' -f1,2)
go install "github.com/ray4go/go-ray/goraygen@go$GO_VER"
``` -->

## Usage

### 1. Annotate Ray Tasks and Actors

Annotate your Ray tasks and actor factories with special comments:

```go
// raytasks
type Tasks struct{}

// rayactors
type Actors struct{}
```

- Use the `// raytasks` comment to mark your Ray tasks struct
- Use the `// rayactors` comment to mark your Ray actor factory struct

### 2. Generate Wrapper Code

Run `goraygen` with the path to your GoRay application package:

```bash
goraygen /path/to/your/package/
```

This generates a `ray_workload_wrappers.go` file in the package directory containing type-safe wrappers for all Ray tasks and actors.

### 3. Use Generated Wrappers

Use the generated wrappers to perform remote call and create new actor:

**Ray Task**

```golang
// Without goraygen wrappers
objRef := ray.RemoteCall("Divide", 16, 5, ray.Option("num_cpus", 2))
res, remainder, err := ray.Get2[int64, int64](objRef)

// With goraygen wrappers
future := Divide(16, 5).Remote(ray.Option("num_cpus", 2))
res, remainder, err := future.Get()
```

For each ray task, a generated wrapper function is created. 

`Remote()` call accepts `ray.Option`s and returns a `Future` object, which can be used to retrieve the result via `future.Get()`

The wrapper function signature matches the original task function, the wrapper function also accepts compatible `Future` parameters.

**Ray Actor**

```golang
// Without goraygen wrappers
counter := ray.NewActor("Counter", 1)
obj := counter.RemoteCall("Incr", 2)
res, err := ray.Get1[int64](obj)
counter.Kill()

// With goraygen wrappers
counter := NewCounter(1).Remote()
future := Counter_Incr(counter, 2).Remote()
res, err := future.Get()
counter.Kill()
```

For actor `Counter`, the generated wrappers include:

- `ActorCounter` type representing the actor handle
- `NewCounter(constructor params).Remote() -> (*ActorCounter, error)` to create a new actor
- `Counter_MethodName(actor *ActorCounter, method params)` functions for each actor method

**Named Actor**

Use `ray.GetTypedActor` generic function to get type-safe actor handle.

```golang
NewCounter(1).Remote(ray.Option("name", "counter"))

counter, err := ray.GetTypedActor[ActorCounter]("counter")
future := Counter_Incr(counter, 2).Remote()
```

### Notes

- `goraygen` only generates wrappers for Ray tasks and actors defined in Golang, not for tasks or actors defined in Python.
- The returned Future objects from wrapper remote call and `ray.Put()` can be passed as parameters to other wrapper remote calls.
  The objectRef returned by `ray.RemoteCall` and `actor.RemoteCall` can't be used as parameters to wrapper remote calls.
- `Cancel()` and `ray.Wait()` are not natively supported on Future types. Use `objectRef.Cancel()` and `ray.Wait()` with the underlying object references via `future.ObjectRef()`.
- Variadic parameters are partially supported in wrapper functions. You can't pass mixed types in variadic parameters (i.e., both concrete values and Future objects).
- Do not manually edit generated files. The generated `ray_workload_wrappers.go` file is overwritten on each run of `goraygen`.

## Examples

See [example application](https://github.com/ray4go/go-ray/tree/master/examples/basic).

## Regeneration

After modifying Ray task or actor definitions, simply re-run `goraygen` to update the wrapper code:

```bash
goraygen /path/to/your/package/
```

## Related Documentation

- [GoRay Documentation](https://github.com/ray4go/go-ray)
- [Ray Core API Reference](https://docs.ray.io/en/latest/ray-core/api/core.html)
