package main

import (
	"fmt"
	"log"

	"asda2/shared/relay"
)

func gmSetProfession(cmd relay.GMCommand) error {
	target, err := gmCommandTarget(cmd)
	if err != nil {
		return err
	}
	classID, err := parseUint16Arg(cmd.Args, "class")
	if err != nil {
		return err
	}
	realLevel, err := parseUint16Arg(cmd.Args, "level")
	if err != nil {
		return err
	}
	if classID > 9 {
		return fmt.Errorf("class must be 1-9")
	}
	if realLevel == 0 || realLevel > 4 {
		return fmt.Errorf("level must be 1-4")
	}
	if !setCharacterClass(target, byte(realLevel), byte(classID)) {
		return fmt.Errorf("failed to set profession for %q", target.Char.Name)
	}
	log.Printf("[GM] %s set %q profession class=%d realLevel=%d encodedLevel=%d",
		cmd.RequestedBy, target.Char.Name, target.Char.Class, target.Char.RealProfessionLevel(), target.Char.ProfessionLevel)
	return nil
}
