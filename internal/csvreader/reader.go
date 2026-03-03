package csvreader

import (
	"encoding/csv"
	"log"
	"os"

	"ai-outreach-engine/internal/models"
)

func ReadCSV(path string) []models.HRMessage {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal("Failed to open CSV:", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Failed to read CSV:", err)
	}

	var results []models.HRMessage

	for i, row := range records {
		if i == 0 {
			continue // skip header
		}

		results = append(results, models.HRMessage{
			HRName:      row[0],
			HREmail:     row[1],
			CompanyName: row[2],
			Website:     row[3],
		})
	}

	return results
}
