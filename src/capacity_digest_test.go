package zenithdtl

import "testing"

func TestCapacityDigestReconcilesOperatorAndChannels(t *testing.T) {
	p, refs, err := seedNetwork()
	if err != nil {
		t.Fatal(err)
	}
	rows := BuildCapacityDigest(p)
	if len(rows) != 4 {
		t.Fatalf("expected four operators, got %d", len(rows))
	}
	var atlas CapacityDigest
	for _, row := range rows {
		if row.Operator == refs.Atlas {
			atlas = row
		}
	}
	if atlas.ChannelCount != 2 || atlas.AssignedLimit <= 0 || atlas.PlannedLimit <= 0 {
		t.Fatalf("invalid initial capacity: %+v", atlas)
	}
	if atlas.OpenExposure != 0 || atlas.AssignedHeadroom != atlas.AssignedLimit {
		t.Fatalf("invalid initial exposure: %+v", atlas)
	}
	_, err = p.OpenObligation(SettlementIntent{Payer: refs.UserA, Beneficiary: refs.UserB, Amount: 25_000, Route: refs.RouteAB})
	if err != nil {
		t.Fatal(err)
	}
	rows = BuildCapacityDigest(p)
	for _, row := range rows {
		if row.Operator != refs.Atlas {
			continue
		}
		if row.OpenExposure <= 25_000 || row.AssignedHeadroom != row.AssignedLimit-row.OpenExposure {
			t.Fatalf("invalid active exposure: %+v", row)
		}
		if row.UtilizationBps <= 0 || row.ChannelHeadroom >= atlas.ChannelHeadroom {
			t.Fatalf("invalid utilization: %+v", row)
		}
		return
	}
	t.Fatal("atlas row missing")
}
