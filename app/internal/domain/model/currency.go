package model

type Currency struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Code      string `gorm:"size:3;uniqueIndex;not null" json:"code"`
	Country   string `gorm:"size:64;not null" json:"country"`
	Name      string `gorm:"size:64;not null" json:"name"`
	Amount    int    `gorm:"not null" json:"amount"`
	IsTracked bool   `gorm:"not null;default:true" json:"isTracked"`
}
