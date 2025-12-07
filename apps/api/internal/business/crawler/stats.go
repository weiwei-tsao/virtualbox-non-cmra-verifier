package crawler

import "github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"

// AggregateSystemStats reduces mailboxes into dashboard stats.
func AggregateSystemStats(mailboxes []model.Mailbox) model.SystemStats {
	var total, commercial, residential int
	var priceSum float64
	byState := make(map[string]int)

	for _, m := range mailboxes {
		if !m.Active {
			continue
		}
		total++
		priceSum += m.Price
		if m.RDI == "Commercial" {
			commercial++
		}
		if m.RDI == "Residential" {
			residential++
		}
		byState[m.AddressRaw.State]++
	}

	var avgPrice float64
	if total > 0 {
		avgPrice = priceSum / float64(total)
	}

	return model.SystemStats{
		TotalMailboxes:   total,
		TotalCommercial:  commercial,
		TotalResidential: residential,
		AvgPrice:         avgPrice,
		ByState:          byState,
	}
}
