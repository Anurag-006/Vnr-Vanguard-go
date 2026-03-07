package utils

import (
	"fmt"
	"strconv"
)

type SectionInfo struct {
	Code     string
	RegStart int
	RegEnd   int
	LatStart int
	LatEnd   int
}

func getSequenceStrings(startIdx, endIdx int) []string {

	if startIdx > endIdx || startIdx < 1 {
		return nil
	}

	// We calculate the exact size needed and allocate it once.
	capacity := (endIdx - startIdx) + 1
	seq := make([]string, 0, capacity)
	
	validChars := "ABCDEFGHJKMNPQRTUVWXYZ"

	for i := startIdx; i <= endIdx; i++ {
		if i <= 99 {
			// Format as 01, 02, ..., 99
			seq = append(seq, fmt.Sprintf("%02d", i))
		} else {
			// JNTUH/VNR Alphanumeric encoding for 100+
			offset := i - 100
			charIndex := offset / 10
			digit := offset % 10

			if charIndex < len(validChars) {
				seq = append(seq, fmt.Sprintf("%c%d", validChars[charIndex], digit))
			}
		}
	}
	return seq
}

// GenerateRollNumbers constructs the full list of roll numbers.
func GenerateRollNumbers(yearStr string, section SectionInfo) ([]string, error) {
	yearInt, err := strconv.Atoi(yearStr)
	if err != nil {
		return nil, fmt.Errorf("invalid year prefix '%s': must be a number", yearStr)
	}

	// Pre-allocate the final slice to save memory
	totalExpected := (section.RegEnd - section.RegStart + 1)
	if section.LatEnd >= section.LatStart {
		totalExpected += (section.LatEnd - section.LatStart + 1)
	}

	rolls := make([]string, 0, totalExpected)

	// 1. Regular Students (e.g., "23" + "071A" + "32" + "01")
	regSeq := getSequenceStrings(section.RegStart, section.RegEnd)
	for _, s := range regSeq {
		rolls = append(rolls, fmt.Sprintf("%s071A%s%s", yearStr, section.Code, s))
	}

	// 2. Lateral Entry Students (e.g., "24" + "075A" + "32" + "01")
	latYear := strconv.Itoa(yearInt + 1)
	latSeq := getSequenceStrings(section.LatStart, section.LatEnd)

	for _, s := range latSeq {
		rolls = append(rolls, fmt.Sprintf("%s075A%s%s", latYear, section.Code, s))
	}

	return rolls, nil
}