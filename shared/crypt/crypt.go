package crypt

// Locale mirrors Asda2CryptHelper locale values.
type Locale uint8

const (
	LocaleAny    Locale = 255 // C# Any = -1, treated as unencoded/default
	LocaleStart  Locale = 0   // English
	LocaleRu     Locale = 1
	LocaleAr     Locale = 2
	LocaleTahadi Locale = 3
	LocaleLos    Locale = 4
)

// initCryptKeys computes derived locale key tables from the two base tables.
// Must be called once at startup (main.go calls it before serve).
//
// Derivation rules from Asda2CryptHelper.InitXorKeys():
//
//	Ru  = Start XOR 228  (0xE4)
//	Ar  = Start XOR 116  (0x74)
//	LOS = Tahadi XOR 103 (0x67)
func InitKeys() {
	deriveFrom(xorKeysStart, 228, &xorKeysRu)
	deriveFrom(xorKeysStart, 116, &xorKeysAr)
	deriveFrom(xorKeysTahadi, 103, &xorKeysLos)
}

func deriveFrom(base [256][]byte, xorByte byte, dst *[256][]byte) {
	for i, src := range base {
		if len(src) == 0 {
			dst[i] = nil
			continue
		}
		derived := make([]byte, len(src))
		for j, b := range src {
			derived[j] = b ^ xorByte
		}
		dst[i] = derived
	}
}

// xorData encrypts/decrypts a buffer in-place.
// Mirrors Asda2CryptHelper.XorData(buf, startIndex, length, Locale.Any, clientLocale).
//
// Layout:
//
//	buf[startIdx]       = keyNum (selects which 512-byte key to use)
//	buf[startIdx+1+i]  ^= key[i]   for i in [0, length-2)
func XorData(buf []byte, startIdx, length int, locale Locale) {
	if length <= 1 || startIdx >= len(buf) {
		return
	}
	keyNum := buf[startIdx]
	var keys *[256][]byte
	switch locale {
	case LocaleRu:
		keys = &xorKeysRu
	case LocaleAr:
		keys = &xorKeysAr
	case LocaleTahadi:
		keys = &xorKeysTahadi
	case LocaleLos:
		keys = &xorKeysLos
	default: // LocaleStart, LocaleAny
		keys = &xorKeysStart
	}
	key := keys[keyNum]
	n := length - 1
	if n > len(key) {
		n = len(key)
	}
	base := startIdx + 1
	for i := 0; i < n; i++ {
		buf[base+i] ^= key[i]
	}
}
