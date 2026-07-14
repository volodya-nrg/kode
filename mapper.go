package other

func ScheduleToDB(item ScheduleItem) ScheduleRecord {
	return ScheduleRecord{
		ScheduleID: item.ScheduleID,
		UserID:     item.UserID,
		DrugName:   item.DrugName,
		Period:     item.Period,
		TTL:        item.TTL,
	}
}

func ScheduleToItem(item ScheduleRecord) ScheduleItem {
	return ScheduleItem{
		ScheduleID: item.ScheduleID,
		UserID:     item.UserID,
		DrugName:   item.DrugName,
		Period:     item.Period,
		TTL:        item.TTL,
	}
}

func NearestDrugsToTransport(itemsSrc []NearestDrugItem) []ResponseNearestDrug {
	items := make([]ResponseNearestDrug, len(itemsSrc))
	for k, v := range itemsSrc {
		items[k] = ResponseNearestDrug{
			DrugName: v.DrugName,
			Time:     v.Time,
		}
	}
	return items
}
