package cost

import "fmt"

// InstancePricing contains real scraped cloud pricing data
// Prices are in USD per hour
type InstancePricing struct {
	InstanceType string
	OnDemand     float64
	Spot         float64 // Average spot price
}

// AWS EC2 Pricing Data - Generated from provided CSV files
var AWSPricing = map[string]map[string]InstancePricing{
	"ap-northeast-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.1070, Spot: 0.0379},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.2140, Spot: 0.0705},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.4280, Spot: 0.1642},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1240, Spot: 0.0337},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.2480, Spot: 0.0674},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.4960, Spot: 0.1616},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.9920, Spot: 0.4014},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1520, Spot: 0.0445},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.3040, Spot: 0.1014},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.6080, Spot: 0.1607},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0136, Spot: 0.0039},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0272, Spot: 0.0075},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0544, Spot: 0.0160},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.1088, Spot: 0.0430},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.2176, Spot: 0.0619},
	},
	"eu-central-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.0970, Spot: 0.0336},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.1940, Spot: 0.0659},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.3880, Spot: 0.1215},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1150, Spot: 0.0356},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.2300, Spot: 0.0622},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.4600, Spot: 0.1323},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.9200, Spot: 0.3328},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1520, Spot: 0.0522},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.3040, Spot: 0.1109},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.6080, Spot: 0.2002},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0120, Spot: 0.0039},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0240, Spot: 0.0072},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0480, Spot: 0.0178},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.0960, Spot: 0.0406},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.1920, Spot: 0.0779},
	},
	"eu-west-3": {
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.4040, Spot: 0.1659},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.2020, Spot: 0.0677},
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.1010, Spot: 0.0357},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.4480, Spot: 0.1419},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.2240, Spot: 0.0756},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1120, Spot: 0.0349},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.8960, Spot: 0.3275},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.5920, Spot: 0.1621},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.2960, Spot: 0.0755},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1480, Spot: 0.0427},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.1888, Spot: 0.0735},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.0944, Spot: 0.0288},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0472, Spot: 0.0166},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0118, Spot: 0.0031},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0236, Spot: 0.0066},
	},
	"sa-east-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.1310, Spot: 0.0400},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.2620, Spot: 0.0851},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.5240, Spot: 0.1575},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.1530, Spot: 0.0384},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.3060, Spot: 0.0312},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.6120, Spot: 0.0612},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 1.2240, Spot: 0.2745},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.2010, Spot: 0.0747},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.4020, Spot: 0.0782},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.8040, Spot: 0.2100},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0168, Spot: 0.0050},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0336, Spot: 0.0106},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0672, Spot: 0.0229},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.1344, Spot: 0.0538},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.2688, Spot: 0.1136},
	},
	"us-east-1": {
		"c5.large":   {InstanceType: "c5.large", OnDemand: 0.0850, Spot: 0.0307},
		"c5.xlarge":  {InstanceType: "c5.xlarge", OnDemand: 0.1700, Spot: 0.0685},
		"c5.2xlarge": {InstanceType: "c5.2xlarge", OnDemand: 0.3400, Spot: 0.1299},
		"m5.large":   {InstanceType: "m5.large", OnDemand: 0.0960, Spot: 0.0360},
		"m5.xlarge":  {InstanceType: "m5.xlarge", OnDemand: 0.1920, Spot: 0.0685},
		"m5.2xlarge": {InstanceType: "m5.2xlarge", OnDemand: 0.3840, Spot: 0.1512},
		"m5.4xlarge": {InstanceType: "m5.4xlarge", OnDemand: 0.7680, Spot: 0.3105},
		"r5.large":   {InstanceType: "r5.large", OnDemand: 0.1260, Spot: 0.0440},
		"r5.xlarge":  {InstanceType: "r5.xlarge", OnDemand: 0.2520, Spot: 0.0861},
		"r5.2xlarge": {InstanceType: "r5.2xlarge", OnDemand: 0.5040, Spot: 0.1801},
		"t3.micro":   {InstanceType: "t3.micro", OnDemand: 0.0104, Spot: 0.0036},
		"t3.small":   {InstanceType: "t3.small", OnDemand: 0.0208, Spot: 0.0074},
		"t3.medium":  {InstanceType: "t3.medium", OnDemand: 0.0416, Spot: 0.0166},
		"t3.large":   {InstanceType: "t3.large", OnDemand: 0.0832, Spot: 0.0335},
		"t3.xlarge":  {InstanceType: "t3.xlarge", OnDemand: 0.1664, Spot: 0.0604},
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
