package controller

import "testing"

func TestHandwrittenGoConsumesGeneratedOctFlow(t *testing.T) {
	tests := []struct {
		name      string
		requested int
		capacity  int
		want      int
	}{
		{name: "accept", requested: 3, capacity: 4, want: 1},
		{name: "reject", requested: 5, capacity: 4, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			machine := fn_Controller_AdmissionController(test.requested, test.capacity)
			machine.__octStep()
			got, complete := machine.__octResult()
			if !complete {
				t.Fatal("generated Oct flow did not complete")
			}
			if got != test.want {
				t.Fatalf("generated Oct flow result = %d, want %d", got, test.want)
			}
		})
	}
}
