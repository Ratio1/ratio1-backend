package model

type JobType int

var jobTypeNames = map[JobType]string{
	1: "ENTRY",
	2: "LOW1",
	3: "LOW2",
	4: "MED1",
	5: "MED2",
	6: "HIGH1",
	7: "HIGH2",
	8: "ULTRA1",
	9: "ULTRA2",

	10: "PGSQL-LOW",
	11: "PGSQL-MED",
	12: "MYSQL-LOW",
	13: "MYSQL-MED",
	14: "NOSQL-LOW",
	15: "NOSQL-MED",

	16: "N-ENTRY",
	17: "N-MED1",
	18: "N-MED2",
	19: "N-HIGH",
	20: "N-ULTRA",

	21: "G-ENTRY-MED1",
	22: "G-ENTRY-MED2",
	23: "G-ENTRY-N-ENTRY",
	24: "G-ENTRY-N-MED1",

	25: "G-MED-MED2",
	26: "G-MED-HIGH1",
	27: "G-MED-HIGH2",
	28: "G-MED-ULTRA1",
	29: "G-MED-N-MED1",
	30: "G-MED-N-MED2",
	31: "G-MED-N-HIGH",

	32: "G-HIGH-HIGH2",
	33: "G-HIGH-ULTRA1",
	34: "G-HIGH-ULTRA2",
	35: "G-HIGH-N-MED2",
	36: "G-HIGH-N-HIGH",
	37: "G-HIGH-N-ULTRA",

	38: "G-ULTRA-ULTRA1",
	39: "G-ULTRA-ULTRA2",
	40: "G-ULTRA-N-ULTRA",

	50: "SERVICE_ENTRY",
	51: "SERVICE_MED1",
	52: "SERVICE_HIGH1",
}

func (j *JobType) GetName() string {
	return jobTypeNames[*j]
}
