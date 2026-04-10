package cli

import "testing"

func TestExtractAllFlag(t *testing.T) {
	showAll, args := ExtractAllFlag([]string{"ls", "-a"})
	if !showAll {
		t.Fatal("expected showAll=true")
	}
	if len(args) != 1 || args[0] != "ls" {
		t.Fatalf("expected args=[ls], got %#v", args)
	}

	showAll, args = ExtractAllFlag([]string{"--all", "kill", "3000"})
	if !showAll {
		t.Fatal("expected showAll=true with --all")
	}
	if len(args) != 2 || args[0] != "kill" || args[1] != "3000" {
		t.Fatalf("unexpected args after flag extraction: %#v", args)
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	code := Execute([]string{"wat"}, false)
	if code != 1 {
		t.Fatalf("expected exit code 1 for unknown command, got %d", code)
	}
}

func TestKillPortsInvalidPort(t *testing.T) {
	code := KillPorts([]string{"abc"}, false)
	if code != 1 {
		t.Fatalf("expected exit code 1 for invalid port, got %d", code)
	}
}
