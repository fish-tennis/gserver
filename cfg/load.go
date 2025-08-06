package cfg

func Process(mgr any, mgrName, messageName, fileName string) error {
	switch messageName {
	case "QuestCfg":
		questAfterLoad(mgr, mgrName, messageName, fileName)
	case "ActivityCfg":
		ActivityAfterLoad(mgr, mgrName, messageName, fileName)
	case "ExchangeCfg":
		exchangeAfterLoad(mgr, mgrName, messageName, fileName)
	}
	return nil
}
