package rtk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ParsedArgs struct {
	Values      map[string]string
	Flags       map[string]bool
	Positionals []string
}

func ParseArgs(argv []string) ParsedArgs {
	args := ParsedArgs{
		Values:      map[string]string{},
		Flags:       map[string]bool{},
		Positionals: []string{},
	}
	for index := 0; index < len(argv); index++ {
		arg := argv[index]
		if !strings.HasPrefix(arg, "--") {
			args.Positionals = append(args.Positionals, arg)
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		if index+1 >= len(argv) || strings.HasPrefix(argv[index+1], "--") {
			args.Flags[key] = true
			continue
		}
		args.Values[key] = argv[index+1]
		index++
	}
	return args
}

func (p ParsedArgs) Has(key string) bool {
	_, okValue := p.Values[key]
	_, okFlag := p.Flags[key]
	return okValue || okFlag
}

func (p ParsedArgs) Value(key string) string {
	return p.Values[key]
}

func Ensure(value string, message string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func ReadJSONStdin(target any) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

type TempSchemaHandle struct {
	File string
	Dir  string
}

func WriteTempSchema(schema any) (*TempSchemaHandle, error) {
	dir, err := os.MkdirTemp("", "roundtable-schema-")
	if err != nil {
		return nil, err
	}
	file := filepath.Join(dir, "schema.json")
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return nil, err
	}
	return &TempSchemaHandle{File: file, Dir: dir}, nil
}

func (h *TempSchemaHandle) Cleanup() {
	if h == nil {
		return
	}
	_ = os.Remove(h.File)
	_ = os.Remove(h.Dir)
}

func PrintJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}
