package types

import "testing"

func TestProfessionLevelEncodingMatchesReferenceFamilies(t *testing.T) {
	tests := []struct {
		name        string
		classID     byte
		realLevel   byte
		encoded     byte
		realDecoded byte
	}{
		{name: "warrior", classID: byte(Asda2ClassOHS), realLevel: 2, encoded: 2, realDecoded: 2},
		{name: "archer", classID: byte(Asda2ClassBow), realLevel: 2, encoded: 13, realDecoded: 2},
		{name: "mage", classID: byte(Asda2ClassHealMage), realLevel: 2, encoded: 24, realDecoded: 2},
		{name: "none", classID: byte(Asda2ClassNone), realLevel: 2, encoded: 0, realDecoded: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncodedProfessionLevel(tt.classID, tt.realLevel); got != tt.encoded {
				t.Fatalf("EncodedProfessionLevel() = %d, want %d", got, tt.encoded)
			}
			if got := RealProfessionLevel(tt.classID, tt.encoded); got != tt.realDecoded {
				t.Fatalf("RealProfessionLevel() = %d, want %d", got, tt.realDecoded)
			}
		})
	}
}

func TestAsda2ClassFamily(t *testing.T) {
	if got := Asda2ClassFamily(byte(Asda2ClassSpear)); got != Asda2ProfessionWarrior {
		t.Fatalf("Spear family = %d, want warrior", got)
	}
	if got := Asda2ClassFamily(byte(Asda2ClassCrossbow)); got != Asda2ProfessionArcher {
		t.Fatalf("Crossbow family = %d, want archer", got)
	}
	if got := Asda2ClassFamily(byte(Asda2ClassAttackMage)); got != Asda2ProfessionMage {
		t.Fatalf("AttackMage family = %d, want mage", got)
	}
	if IsAsda2Class(10) {
		t.Fatalf("class 10 should not be a valid Asda2 class")
	}
}
