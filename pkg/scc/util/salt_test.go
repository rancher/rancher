package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewSaltGen(t *testing.T) {
	t.Parallel()
	t.Run("DefaultInitialization", func(t *testing.T) {
		// Test with nil timeIn and nil charsetIn (should use defaults)
		sg := NewSaltGen(nil, nil)

		assert.NotNil(t, sg)
		assert.NotNil(t, sg.randSrc)
		assert.Equal(t, charset, sg.saltCharset)
		assert.Equal(t, len(charset), sg.charsetLen)
	})

	t.Run("CustomTimeAndCharset", func(t *testing.T) {
		// Test with custom timeIn and charsetIn
		fixedTime := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
		customCharset := "0123"

		sg := NewSaltGen(&fixedTime, &customCharset)

		assert.NotNil(t, sg)
		assert.NotNil(t, sg.randSrc)
		assert.Equal(t, customCharset, sg.saltCharset)
		assert.Equal(t, len(customCharset), sg.charsetLen)

		// Verify that a fixed time seed produces a deterministic sequence for GenerateCharacter
		// This uses a new SaltGen with the same fixed seed
		sgFixedSeed := NewSaltGen(&fixedTime, &customCharset)
		expectedChar1 := sgFixedSeed.GenerateCharacter()
		expectedChar2 := sgFixedSeed.GenerateCharacter()

		sgSameFixedSeed := NewSaltGen(&fixedTime, &customCharset)
		actualChar1 := sgSameFixedSeed.GenerateCharacter()
		actualChar2 := sgSameFixedSeed.GenerateCharacter()

		assert.Equal(t, expectedChar1, actualChar1)
		assert.Equal(t, expectedChar2, actualChar2)
	})
}

func TestGenerateCharacter(t *testing.T) {
	// Use a fixed seed for deterministic testing of character generation
	fixedTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	defaultCharset := charset // Use the global charset constant

	sg := NewSaltGen(&fixedTime, &defaultCharset)

	// Generate a few characters and ensure they are within the charset
	for i := 0; i < 100; i++ { // Test multiple generations
		char := sg.GenerateCharacter()
		assert.Contains(t, sg.saltCharset, string(char))
	}

	t.Run("DeterministicSequence", func(t *testing.T) {
		// Verify that with the same seed, characters are generated deterministically
		sg1 := NewSaltGen(&fixedTime, &defaultCharset)
		sg2 := NewSaltGen(&fixedTime, &defaultCharset) // Same seed

		for i := 0; i < 5; i++ { // Compare a few characters
			char1 := sg1.GenerateCharacter()
			char2 := sg2.GenerateCharacter()
			assert.Equal(t, char1, char2)
		}
	})
}

func TestGenerateSalt(t *testing.T) {
	// Use a fixed seed for deterministic testing of salt generation
	fixedTime := time.Date(2001, 2, 3, 4, 5, 6, 7, time.UTC)
	defaultCharset := charset // Use the global charset constant

	sg := NewSaltGen(&fixedTime, &defaultCharset)

	t.Parallel()
	t.Run("SaltLength", func(t *testing.T) {
		salt := sg.GenerateSalt()
		expectedLength := 8
		assert.Len(t, salt, expectedLength)
	})

	t.Run("CharactersFromCharset", func(t *testing.T) {
		salt := sg.GenerateSalt()
		for _, char := range salt {
			assert.Contains(t, sg.saltCharset, string(char))
		}
	})

	t.Run("DeterministicSaltWithFixedSeed", func(t *testing.T) {
		// Two generators with the same fixed seed should produce the same salt sequence
		sg1 := NewSaltGen(&fixedTime, &defaultCharset)
		sg2 := NewSaltGen(&fixedTime, &defaultCharset)

		salt1 := sg1.GenerateSalt()
		salt2 := sg2.GenerateSalt()

		assert.Equal(t, salt1, salt2)

		// Generate another pair to ensure it's not just the first one
		salt1_2 := sg1.GenerateSalt()
		salt2_2 := sg2.GenerateSalt()
		assert.Equal(t, salt1_2, salt2_2)
	})

	t.Run("DifferentSaltsWithDifferentSeeds", func(t *testing.T) {
		// Two generators with different seeds should produce different salts (highly probable)
		time1 := time.Date(2002, 3, 4, 0, 0, 0, 0, time.UTC)
		time2 := time.Date(2002, 3, 4, 0, 0, 0, 1, time.UTC) // Slightly different time

		sg1 := NewSaltGen(&time1, &defaultCharset)
		sg2 := NewSaltGen(&time2, &defaultCharset)

		salt1 := sg1.GenerateSalt()
		salt2 := sg2.GenerateSalt()

		assert.NotEqual(t, salt1, salt2)
	})

	t.Run("DifferentSaltsOnConsecutiveCalls", func(t *testing.T) {
		// When using the system's real time for seeding (which is what nil `timeIn` does),
		// consecutive calls to `NewSaltGen` will likely result in different seeds
		// and thus different salt sequences.
		// Here, we're testing the `GenerateSalt` method's behavior with a *single* SaltGen instance
		// that's meant to produce different salts each time.

		// Using a fixed seed for predictability in test
		sg := NewSaltGen(&fixedTime, &defaultCharset)

		salt1 := sg.GenerateSalt()
		salt2 := sg.GenerateSalt()

		assert.NotEqual(t, salt1, salt2)
	})
}
