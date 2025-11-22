# goraygen

`goraygen` is a code generation tool provided by GoRay that generates type-safe wrapper APIs for Ray tasks and actors.

## Features

`goraygen` solves the following issues when using GoRay:

- **Compile-time type checking**: The original `ray.RemoteCall()` and `ray.NewActor()` use strings to specify task/actor names with parameters of type `any`, preventing compile-time verification of names and parameter types
- **IDE support**: Generated wrapper code provides full code completion, type hints, and other IDE features
- **Type-safe return values**: Returned Future objects have explicit type information without requiring manual type specification
- **Better developer experience**: Calling Ray tasks/actors through wrapper APIs achieves compile-time type safety and is IDE-friendly

## Installation

```bash
go install "github.com/ray4go/goraygen@latest"
```

<!-- If you get errors about Go version compatibility, use the following command to install the appropriate version based on your Go version:

```bash
GO_VER=$(go env GOVERSION | sed 's/go//' | cut -d'.' -f1,2)
go install "github.com/ray4go/go-ray/goraygen@go$GO_VER"
``` -->

## Usage

### 1. Annotate Ray Tasks and Actors

Use comments to annotate ray tasks struct and ray actor factory struct in your code:

```go
// raytasks
type tasks struct{}

// rayactors
type actors struct{}
```

- Use `// raytasks` comment to annotate the ray tasks struct
- Use `// rayactors` comment to annotate the ray actor factory struct

### 2. Generate Wrapper Code

Run the following command in the package directory containing Ray tasks and actors:

```bash
goraygen /path/to/your/package/
```

`goraygen` will generate a `ray_workloads_wrapper.go` file in the package directory containing type-safe wrappers for all Ray tasks and actors.

### 3. Use Generated Wrappers

The generated wrappers can be used directly, providing type-safe APIs:

**Calling Ray Tasks:**

```golang
// Original way
objRef := ray.RemoteCall("Divide", 16, 5, ray.Option("num_cpus", 2))
res, remainder, err := ray.Get2[int64, int64](objRef)

// Using goraygen generated wrapper
future := Divide(16, 5).Remote(ray.Option("num_cpus", 2))
res, remainder, err := future.Get()
```

**Creating and Calling Ray Actors:**

```golang
// Original way
cnt := ray.NewActor("Counter", 1)
obj := cnt.RemoteCall("Incr", 2)
var res int
err := obj.GetInto(&res)

// Using goraygen generated wrapper
counter := NewCounter(1).Remote()
future := Counter_Incr(counter, 2).Remote()
res, err := future.Get()
```

## Notes

- The generated `ray_workloads_wrapper.go` file will be overwritten each time `goraygen` is run
- Do not manually edit the generated file, as modifications will be lost on the next generation
- Ensure that Ray task and actor methods are public (capitalized first letter)
- Supports Go language features such as variadic parameters and multiple return values

## Examples

For complete example code, refer to:

- [examples/app.go](../examples/app.go) - Basic usage example
- [examples/ray_workloads_wrapper.go](../examples/ray_workloads_wrapper.go) - Generated wrapper code example

## Regeneration

After modifying Ray task or actor definitions, re-run the following command to update the wrapper code:

```bash
goraygen /path/to/your/package/
```

## Related Documentation

- [GoRay Main Documentation](https://github.com/ray4go/go-ray)
- [Ray Official API Documentation](https://docs.ray.io/en/latest/ray-core/api/core.html)
