package cost

import "fmt"

// InstancePricing contains real scraped cloud pricing data
// Prices are in USD per hour
type InstancePricing struct {
	InstanceType   string
	OnDemand       float64 // USD per hour for the full instance
	Spot           float64 // Average spot price for the full instance
	OnDemandPerCPU float64 // USD per hour per vCPU (on-demand)
	SpotPerCPU     float64 // USD per hour per vCPU (spot)
	OnDemandPerGiB float64 // USD per hour per GiB memory (on-demand)
	SpotPerGiB     float64 // USD per hour per GiB memory (spot)
}

// AWS EC2 Pricing Data - Generated from provided CSV files
var AWSPricing = map[string]map[string]InstancePricing{
	"ap-northeast-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.1070, Spot: 0.0379, OnDemandPerCPU: 0.0535, SpotPerCPU: 0.0194, OnDemandPerGiB: 0.0268, SpotPerGiB: 0.0097},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.2140, Spot: 0.0705, OnDemandPerCPU: 0.0535, SpotPerCPU: 0.0181, OnDemandPerGiB: 0.0268, SpotPerGiB: 0.0090},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.4280, Spot: 0.1642, OnDemandPerCPU: 0.0535, SpotPerCPU: 0.0205, OnDemandPerGiB: 0.0268, SpotPerGiB: 0.0102},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1240, Spot: 0.0337, OnDemandPerCPU: 0.0620, SpotPerCPU: 0.0184, OnDemandPerGiB: 0.0155, SpotPerGiB: 0.0046},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.2480, Spot: 0.0674, OnDemandPerCPU: 0.0620, SpotPerCPU: 0.0180, OnDemandPerGiB: 0.0155, SpotPerGiB: 0.0045},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.4960, Spot: 0.1616, OnDemandPerCPU: 0.0620, SpotPerCPU: 0.0209, OnDemandPerGiB: 0.0155, SpotPerGiB: 0.0052},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.9920, Spot: 0.4014, OnDemandPerCPU: 0.0620, SpotPerCPU: 0.0242, OnDemandPerGiB: 0.0155, SpotPerGiB: 0.0060},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1520, Spot: 0.0445, OnDemandPerCPU: 0.0760, SpotPerCPU: 0.0233, OnDemandPerGiB: 0.0095, SpotPerGiB: 0.0029},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.3040, Spot: 0.1014, OnDemandPerCPU: 0.0760, SpotPerCPU: 0.0269, OnDemandPerGiB: 0.0095, SpotPerGiB: 0.0034},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.6080, Spot: 0.1607, OnDemandPerCPU: 0.0760, SpotPerCPU: 0.0219, OnDemandPerGiB: 0.0095, SpotPerGiB: 0.0027},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0136, Spot: 0.0039, OnDemandPerCPU: 0.0068, SpotPerCPU: 0.0021, OnDemandPerGiB: 0.0136, SpotPerGiB: 0.0041},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0272, Spot: 0.0075, OnDemandPerCPU: 0.0136, SpotPerCPU: 0.0040, OnDemandPerGiB: 0.0136, SpotPerGiB: 0.0040},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0544, Spot: 0.0160, OnDemandPerCPU: 0.0272, SpotPerCPU: 0.0084, OnDemandPerGiB: 0.0136, SpotPerGiB: 0.0042},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.1088, Spot: 0.0430, OnDemandPerCPU: 0.0544, SpotPerCPU: 0.0222, OnDemandPerGiB: 0.0136, SpotPerGiB: 0.0055},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.2176, Spot: 0.0619, OnDemandPerCPU: 0.0544, SpotPerCPU: 0.0166, OnDemandPerGiB: 0.0136, SpotPerGiB: 0.0041},
	},
	"eu-central-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.0970, Spot: 0.0336, OnDemandPerCPU: 0.0485, SpotPerCPU: 0.0168, OnDemandPerGiB: 0.0243, SpotPerGiB: 0.0084},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.1940, Spot: 0.0659, OnDemandPerCPU: 0.0485, SpotPerCPU: 0.0165, OnDemandPerGiB: 0.0243, SpotPerGiB: 0.0082},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.3880, Spot: 0.1215, OnDemandPerCPU: 0.0485, SpotPerCPU: 0.0152, OnDemandPerGiB: 0.0243, SpotPerGiB: 0.0076},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1150, Spot: 0.0356, OnDemandPerCPU: 0.0575, SpotPerCPU: 0.0178, OnDemandPerGiB: 0.0144, SpotPerGiB: 0.0045},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.2300, Spot: 0.0622, OnDemandPerCPU: 0.0575, SpotPerCPU: 0.0156, OnDemandPerGiB: 0.0144, SpotPerGiB: 0.0039},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.4600, Spot: 0.1323, OnDemandPerCPU: 0.0575, SpotPerCPU: 0.0165, OnDemandPerGiB: 0.0144, SpotPerGiB: 0.0041},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.9200, Spot: 0.3328, OnDemandPerCPU: 0.0575, SpotPerCPU: 0.0208, OnDemandPerGiB: 0.0144, SpotPerGiB: 0.0052},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1520, Spot: 0.0522, OnDemandPerCPU: 0.0760, SpotPerCPU: 0.0261, OnDemandPerGiB: 0.0095, SpotPerGiB: 0.0033},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.3040, Spot: 0.1109, OnDemandPerCPU: 0.0760, SpotPerCPU: 0.0277, OnDemandPerGiB: 0.0095, SpotPerGiB: 0.0035},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.6080, Spot: 0.2002, OnDemandPerCPU: 0.0760, SpotPerCPU: 0.0250, OnDemandPerGiB: 0.0095, SpotPerGiB: 0.0031},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0120, Spot: 0.0039, OnDemandPerCPU: 0.0060, SpotPerCPU: 0.0020, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0039},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0240, Spot: 0.0072, OnDemandPerCPU: 0.0060, SpotPerCPU: 0.0018, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0036},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0480, Spot: 0.0178, OnDemandPerCPU: 0.0120, SpotPerCPU: 0.0045, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0045},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.0960, Spot: 0.0406, OnDemandPerCPU: 0.0120, SpotPerCPU: 0.0051, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0051},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.1920, Spot: 0.0779, OnDemandPerCPU: 0.0120, SpotPerCPU: 0.0049, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0049},
	},
	"eu-west-3": {
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.4040, Spot: 0.1659, OnDemandPerCPU: 0.0505, SpotPerCPU: 0.0207, OnDemandPerGiB: 0.0253, SpotPerGiB: 0.0104},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.2020, Spot: 0.0677, OnDemandPerCPU: 0.0505, SpotPerCPU: 0.0169, OnDemandPerGiB: 0.0253, SpotPerGiB: 0.0085},
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.1010, Spot: 0.0357, OnDemandPerCPU: 0.0505, SpotPerCPU: 0.0179, OnDemandPerGiB: 0.0253, SpotPerGiB: 0.0089},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.4480, Spot: 0.1419, OnDemandPerCPU: 0.0560, SpotPerCPU: 0.0177, OnDemandPerGiB: 0.0140, SpotPerGiB: 0.0044},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.2240, Spot: 0.0756, OnDemandPerCPU: 0.0560, SpotPerCPU: 0.0189, OnDemandPerGiB: 0.0140, SpotPerGiB: 0.0047},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1120, Spot: 0.0349, OnDemandPerCPU: 0.0560, SpotPerCPU: 0.0175, OnDemandPerGiB: 0.0140, SpotPerGiB: 0.0044},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.8960, Spot: 0.3275, OnDemandPerCPU: 0.0560, SpotPerCPU: 0.0205, OnDemandPerGiB: 0.0140, SpotPerGiB: 0.0051},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.5920, Spot: 0.1621, OnDemandPerCPU: 0.0740, SpotPerCPU: 0.0203, OnDemandPerGiB: 0.0093, SpotPerGiB: 0.0025},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.2960, Spot: 0.0755, OnDemandPerCPU: 0.0740, SpotPerCPU: 0.0189, OnDemandPerGiB: 0.0093, SpotPerGiB: 0.0024},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1480, Spot: 0.0427, OnDemandPerCPU: 0.0740, SpotPerCPU: 0.0214, OnDemandPerGiB: 0.0093, SpotPerGiB: 0.0027},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.1888, Spot: 0.0735, OnDemandPerCPU: 0.0118, SpotPerCPU: 0.0046, OnDemandPerGiB: 0.0118, SpotPerGiB: 0.0046},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.0944, Spot: 0.0288, OnDemandPerCPU: 0.0118, SpotPerCPU: 0.0036, OnDemandPerGiB: 0.0118, SpotPerGiB: 0.0036},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0472, Spot: 0.0166, OnDemandPerCPU: 0.0118, SpotPerCPU: 0.0042, OnDemandPerGiB: 0.0118, SpotPerGiB: 0.0042},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0118, Spot: 0.0031, OnDemandPerCPU: 0.0059, SpotPerCPU: 0.0016, OnDemandPerGiB: 0.0118, SpotPerGiB: 0.0031},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0236, Spot: 0.0066, OnDemandPerCPU: 0.0059, SpotPerCPU: 0.0017, OnDemandPerGiB: 0.0118, SpotPerGiB: 0.0033},
	},
	"sa-east-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.1310, Spot: 0.0400, OnDemandPerCPU: 0.0655, SpotPerCPU: 0.0200, OnDemandPerGiB: 0.0328, SpotPerGiB: 0.0100},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.2620, Spot: 0.0851, OnDemandPerCPU: 0.0655, SpotPerCPU: 0.0213, OnDemandPerGiB: 0.0328, SpotPerGiB: 0.0106},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.5240, Spot: 0.1575, OnDemandPerCPU: 0.0655, SpotPerCPU: 0.0197, OnDemandPerGiB: 0.0328, SpotPerGiB: 0.0098},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1530, Spot: 0.0384, OnDemandPerCPU: 0.0765, SpotPerCPU: 0.0192, OnDemandPerGiB: 0.0191, SpotPerGiB: 0.0048},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.3060, Spot: 0.0312, OnDemandPerCPU: 0.0765, SpotPerCPU: 0.0078, OnDemandPerGiB: 0.0191, SpotPerGiB: 0.0020},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.6120, Spot: 0.0612, OnDemandPerCPU: 0.0765, SpotPerCPU: 0.0077, OnDemandPerGiB: 0.0191, SpotPerGiB: 0.0019},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 1.2240, Spot: 0.2745, OnDemandPerCPU: 0.0765, SpotPerCPU: 0.0172, OnDemandPerGiB: 0.0191, SpotPerGiB: 0.0043},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.2010, Spot: 0.0747, OnDemandPerCPU: 0.1005, SpotPerCPU: 0.0374, OnDemandPerGiB: 0.0126, SpotPerGiB: 0.0047},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.4020, Spot: 0.0782, OnDemandPerCPU: 0.1005, SpotPerCPU: 0.0196, OnDemandPerGiB: 0.0126, SpotPerGiB: 0.0024},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.8040, Spot: 0.2100, OnDemandPerCPU: 0.1005, SpotPerCPU: 0.0263, OnDemandPerGiB: 0.0126, SpotPerGiB: 0.0033},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0168, Spot: 0.0050, OnDemandPerCPU: 0.0084, SpotPerCPU: 0.0025, OnDemandPerGiB: 0.0168, SpotPerGiB: 0.0050},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0336, Spot: 0.0106, OnDemandPerCPU: 0.0084, SpotPerCPU: 0.0027, OnDemandPerGiB: 0.0168, SpotPerGiB: 0.0053},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0672, Spot: 0.0229, OnDemandPerCPU: 0.0168, SpotPerCPU: 0.0057, OnDemandPerGiB: 0.0168, SpotPerGiB: 0.0057},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.1344, Spot: 0.0538, OnDemandPerCPU: 0.0168, SpotPerCPU: 0.0067, OnDemandPerGiB: 0.0168, SpotPerGiB: 0.0067},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.2688, Spot: 0.1136, OnDemandPerCPU: 0.0168, SpotPerCPU: 0.0071, OnDemandPerGiB: 0.0168, SpotPerGiB: 0.0071},
	},
	"us-east-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.0850, Spot: 0.0307, OnDemandPerCPU: 0.0425, SpotPerCPU: 0.0154, OnDemandPerGiB: 0.0213, SpotPerGiB: 0.0077},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.1700, Spot: 0.0685, OnDemandPerCPU: 0.0425, SpotPerCPU: 0.0173, OnDemandPerGiB: 0.0213, SpotPerGiB: 0.0087},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.3400, Spot: 0.1299, OnDemandPerCPU: 0.0425, SpotPerCPU: 0.0156, OnDemandPerGiB: 0.0213, SpotPerGiB: 0.0078},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.0960, Spot: 0.0360, OnDemandPerCPU: 0.0480, SpotPerCPU: 0.0181, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0045},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.1920, Spot: 0.0685, OnDemandPerCPU: 0.0480, SpotPerCPU: 0.0173, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0043},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.3840, Spot: 0.1512, OnDemandPerCPU: 0.0480, SpotPerCPU: 0.0183, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0046},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.7680, Spot: 0.3105, OnDemandPerCPU: 0.0480, SpotPerCPU: 0.0191, OnDemandPerGiB: 0.0120, SpotPerGiB: 0.0048},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1260, Spot: 0.0440, OnDemandPerCPU: 0.0630, SpotPerCPU: 0.0233, OnDemandPerGiB: 0.0079, SpotPerGiB: 0.0029},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.2520, Spot: 0.0861, OnDemandPerCPU: 0.0630, SpotPerCPU: 0.0216, OnDemandPerGiB: 0.0079, SpotPerGiB: 0.0027},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.5040, Spot: 0.1801, OnDemandPerCPU: 0.0630, SpotPerCPU: 0.0231, OnDemandPerGiB: 0.0079, SpotPerGiB: 0.0029},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0104, Spot: 0.0036, OnDemandPerCPU: 0.0052, SpotPerCPU: 0.0019, OnDemandPerGiB: 0.0104, SpotPerGiB: 0.0037},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0208, Spot: 0.0074, OnDemandPerCPU: 0.0104, SpotPerCPU: 0.0038, OnDemandPerGiB: 0.0104, SpotPerGiB: 0.0038},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0416, Spot: 0.0166, OnDemandPerCPU: 0.0208, SpotPerCPU: 0.0083, OnDemandPerGiB: 0.0104, SpotPerGiB: 0.0042},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.0832, Spot: 0.0335, OnDemandPerCPU: 0.0416, SpotPerCPU: 0.0164, OnDemandPerGiB: 0.0104, SpotPerGiB: 0.0041},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.1664, Spot: 0.0604, OnDemandPerCPU: 0.0416, SpotPerCPU: 0.0147, OnDemandPerGiB: 0.0104, SpotPerGiB: 0.0037},
	},
}

// GetInstanceCost returns the hourly cost for an AWS instance
func GetInstanceCost(region, instanceType, lifecycle string) (float64, error) {
	regionPricing, ok := AWSPricing[region]
	if !ok {
		return 0, fmt.Errorf("pricing data not available for region: %s", region)
	}

	instance, ok := regionPricing[instanceType]
	if !ok {
		return 0, fmt.Errorf("pricing data not available for instance type %s in region %s", instanceType, region)
	}

	if lifecycle == "spot" {
		return instance.Spot, nil
	}
	return instance.OnDemand, nil
}
