package main

import (
	"testing"
)

func TestExtractFilePath_Absolute(t *testing.T) {
	input := `[linters_context] typechecking error: D:\\dev\\DeepRefactor\\testdata\\mistakes.go:21:6: y declared and not used`
	expected := `D:\dev\DeepRefactor\testdata\mistakes.go`

	result := ExtractFilePath(input)
	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestExtractFilePath_UnixAbsolute(t *testing.T) {
	input := `/home/user/project/file.go:10:5: some error`
	expected := `/home/user/project/file.go`

	result := ExtractFilePath(input)
	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestExtractFilePath_Relative(t *testing.T) {
	input := `testdata\mistakes.go:9:2: S1021: should merge variable declaration with assignment on next line (gosimple)
        var x int
        ^
testdata\mistakes.go:24:2: S1021: should merge variable declaration with assignment on next line (gosimple)
        var y string
        ^`
	expected := `testdata\mistakes.go`

	result := ExtractFilePath(input)
	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestExtractFilePath_NoFilePath(t *testing.T) {
	input := `No file path in this string`
	expected := ""

	result := ExtractFilePath(input)
	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestExtractFilePath_UnixStylePath(t *testing.T) {
	input := `/home/user/project/file.go:10:5: some error`
	expected := `/home/user/project/file.go`

	result := ExtractFilePath(input)
	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestExtractFilePath_MultiplePaths(t *testing.T) {
	input := `testdata\mistakes.go:9:2: S1021: should merge variable declaration with assignment on next line (gosimple)
        var x int
        ^
D:\dev\DeepRefactor\testdata\mistakes.go:24:2: S1021: should merge variable declaration with assignment on next line (gosimple)
        var y string
        ^`
	expected := `testdata\mistakes.go`

	result := ExtractFilePath(input)
	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestExtractFilePath_UnixMultiplePaths(t *testing.T) {
	input := `project/file.go:9:2: S1021: should merge variable declaration with assignment on next line (gosimple)
        var x int
        ^
/home/user/project/file.go:24:2: S1021: should merge variable declaration with assignment on next line (gosimple)
        var y string
        ^`
	expected := `project/file.go`

	result := ExtractFilePath(input)
	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}
