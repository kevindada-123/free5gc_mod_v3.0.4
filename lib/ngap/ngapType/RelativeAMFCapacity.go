package ngapType

// Need to import "free5gc/lib/aper" if it uses "aper"

type RelativeAMFCapacity struct {
	Value int64 `aper:"valueLB:0,valueUB:255"`
}