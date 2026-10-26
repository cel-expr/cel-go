module cel.dev/cel-go/policy

go 1.26.10

require (
	cel.dev/cel-go v0.26.1
	cel.dev/cel-go/tools v0.0.0-20251023215754-a36d461be521
	github.com/google/go-cmp v0.7.0
	go.yaml.in/yaml/v3 v3.0.4
	google.golang.org/protobuf v1.36.10
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250311190419-81fb87f6b8bf // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250311190419-81fb87f6b8bf // indirect
)

replace cel.dev/cel-go => ../.

replace cel.dev/cel-go/tools => ../tools/.
