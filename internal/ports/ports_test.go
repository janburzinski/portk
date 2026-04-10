package ports

import "testing"

func TestParseLsofOutput_FiltersAndSorts(t *testing.T) {
	output := `COMMAND   PID USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node     1111 jan   21u  IPv4 0x1234      0t0  TCP *:3000 (LISTEN)
launchd  2222 root  12u  IPv4 0x1235      0t0  TCP *:80 (LISTEN)
node     3333 jan   22u  IPv4 0x1236      0t0  TCP *:3000 (LISTEN)
python3  4444 jan   23u  IPv4 0x1237      0t0  TCP *:5000 (LISTEN)
`

	devOnly := parseLsofOutput(output, false)
	if len(devOnly) != 2 {
		t.Fatalf("expected 2 dev ports, got %d", len(devOnly))
	}
	if devOnly[0].Port != 3000 || devOnly[1].Port != 5000 {
		t.Fatalf("expected sorted ports [3000, 5000], got [%d, %d]", devOnly[0].Port, devOnly[1].Port)
	}

	all := parseLsofOutput(output, true)
	if len(all) != 3 {
		t.Fatalf("expected 3 ports with showAll=true, got %d", len(all))
	}
	if all[0].Port != 80 || all[1].Port != 3000 || all[2].Port != 5000 {
		t.Fatalf("expected sorted ports [80, 3000, 5000], got [%d, %d, %d]", all[0].Port, all[1].Port, all[2].Port)
	}
}

func TestIsDevProcess(t *testing.T) {
	if !isDevProcess("node") {
		t.Fatal("expected node to be treated as dev process")
	}
	if !isDevProcess("node-helper") {
		t.Fatal("expected prefix match for truncated command")
	}
	if isDevProcess("systemd") {
		t.Fatal("did not expect systemd to be treated as dev process")
	}
}
