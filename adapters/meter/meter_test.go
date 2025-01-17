package meter

import (
	"testing"
	"time"

	"github.com/mariuszjedrzejewski/iec62056/iec"
)

var portSettings = iec.PortSettings{
	PortName: "/dev/ttyUSB0",
}

func TestGet(t *testing.T) {
	ps := iec.NewDefaultSettings()
	m := &Meter{
		PortSettings: ps,
		PortName:     "/dev/ttyUSB0",
	}
	msm, err := m.Get(nil)
	if err != nil {
		t.Fatalf("Get failed, error: %s", err.Error())
	}
	t.Logf("Measurement: %v", *msm)
}

func TestGetTwo(t *testing.T) {
	ps := iec.NewDefaultSettings()
	m := &Meter{
		PortSettings: ps,
		PortName:     "/dev/ttyUSB0",
	}
	msm, err := m.Get(nil)
	if err != nil {
		t.Errorf("Get failed, error: %s", err.Error())
	} else {
		t.Logf("Measurement 1: %v", *msm)
	}

	// Try again.
	var st = 5 * time.Second
	time.Sleep(st)
	msm, err = m.Get(nil)
	if err != nil {
		t.Errorf("Get failed, error: %s", err.Error())
	} else {
		t.Logf("Measurement 2: %v", *msm)
	}

	// Try again.
	time.Sleep(st)
	msm, err = m.Get(nil)
	if err != nil {
		t.Errorf("Get failed, error: %s", err.Error())
	} else {
		t.Logf("Measurement 3: %v", *msm)
	}
}
