package types

type Asda2Class byte

const (
	Asda2ClassNone        Asda2Class = 0
	Asda2ClassOHS         Asda2Class = 1
	Asda2ClassSpear       Asda2Class = 2
	Asda2ClassTHS         Asda2Class = 3
	Asda2ClassCrossbow    Asda2Class = 4
	Asda2ClassBow         Asda2Class = 5
	Asda2ClassBalista     Asda2Class = 6
	Asda2ClassAttackMage  Asda2Class = 7
	Asda2ClassSupportMage Asda2Class = 8
	Asda2ClassHealMage    Asda2Class = 9
)

type Asda2ProfessionFamily byte

const (
	Asda2ProfessionNone Asda2ProfessionFamily = iota
	Asda2ProfessionWarrior
	Asda2ProfessionArcher
	Asda2ProfessionMage
)

func Asda2ClassFamily(classID byte) Asda2ProfessionFamily {
	switch Asda2Class(classID) {
	case Asda2ClassOHS, Asda2ClassSpear, Asda2ClassTHS:
		return Asda2ProfessionWarrior
	case Asda2ClassCrossbow, Asda2ClassBow, Asda2ClassBalista:
		return Asda2ProfessionArcher
	case Asda2ClassAttackMage, Asda2ClassSupportMage, Asda2ClassHealMage:
		return Asda2ProfessionMage
	default:
		return Asda2ProfessionNone
	}
}

func EncodedProfessionLevel(classID, realLevel byte) byte {
	if realLevel == 0 {
		return 0
	}
	switch Asda2ClassFamily(classID) {
	case Asda2ProfessionWarrior:
		return realLevel
	case Asda2ProfessionArcher:
		return realLevel + 11
	case Asda2ProfessionMage:
		return realLevel + 22
	default:
		return 0
	}
}

func RealProfessionLevel(classID, professionLevel byte) byte {
	switch Asda2ClassFamily(classID) {
	case Asda2ProfessionWarrior:
		return professionLevel
	case Asda2ProfessionArcher:
		if professionLevel < 11 {
			return 0
		}
		return professionLevel - 11
	case Asda2ProfessionMage:
		if professionLevel < 22 {
			return 0
		}
		return professionLevel - 22
	default:
		return 0
	}
}

func IsAsda2Class(classID byte) bool {
	return Asda2ClassFamily(classID) != Asda2ProfessionNone
}
