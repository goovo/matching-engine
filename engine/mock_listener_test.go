package engine

type MockTrade struct {
	MakerID string
	TakerID string
	Price   int64
	Amount  int64
}

type MockListener struct {
	Trades []MockTrade
}

func (l *MockListener) OnTrade(makerID, takerID string, side Side, price, amount int64) {
	l.Trades = append(l.Trades, MockTrade{
		MakerID: makerID,
		TakerID: takerID,
		Price:   price,
		Amount:  amount,
	})
}

func (l *MockListener) OnOrderAccepted(id string) {}
func (l *MockListener) OnOrderCancelled(id string) {}
