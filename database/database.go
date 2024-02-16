package database

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/everFinance/goar/types"
	"github.com/liteseed/bungo/schema"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Database struct {
	Db *gorm.DB
}

func (db *Database) Migrate() error {
	err := db.Db.AutoMigrate(&schema.Order{}, &schema.OnChainTx{}, &schema.OrderStatistic{})
	return err
}

func (db *Database) InsertOrder(order schema.Order) error {
	return db.Db.Create(&order).Error
}

func (w *Database) GetOpenOrders(itemId string) (schema.Order, error) {
	res := schema.Order{}
	err := w.Db.Model(&schema.Order{}).Where("item_id = ? and payment_status = ?", itemId, schema.UnPayment).Last(&res).Error
	return res, err
}

func (w *Database) GetExpiredOrders() ([]schema.Order, error) {
	now := time.Now().Unix()
	ords := make([]schema.Order, 0, 10)
	err := w.Db.Model(&schema.Order{}).Where("payment_status = ? and payment_expired_time < ?", schema.UnPayment, now).Find(&ords).Error
	return ords, err
}

func (w *Database) ExistPaidOrd(itemId string) bool {
	ord := &schema.Order{}
	err := w.Db.Model(&schema.Order{}).Where("item_id = ? and payment_status = ?", itemId, schema.SuccPayment).First(ord).Error
	return err == gorm.ErrRecordNotFound
}

func (w *Database) IsLatestUnpaidOrd(itemId string, CurExpiredTime int64) bool {
	ord := &schema.Order{}
	err := w.Db.Model(&schema.Order{}).Where("item_id = ? and payment_status = ? and payment_expired_time > ?", itemId, schema.UnPayment, CurExpiredTime).First(ord).Error
	return err == gorm.ErrRecordNotFound
}

func (w *Database) UpdateOrdToExpiredStatus(id uint) error {
	data := make(map[string]interface{})
	data["payment_status"] = schema.ExpiredPayment
	data["on_chain_status"] = schema.FailedOnChain
	return w.Db.Model(&schema.Order{}).Where("id = ?", id).Updates(data).Error
}

func (w *Database) UpdateOrderPay(id uint, everHash string, paymentStatus string, tx *gorm.DB) error {
	db := w.Db
	if tx != nil {
		db = tx
	}
	data := make(map[string]interface{})
	data["payment_status"] = paymentStatus
	data["payment_id"] = everHash
	return db.Model(&schema.Order{}).Where("id = ?", id).Updates(data).Error
}

func (w *Database) GetNeedOnChainOrders() ([]schema.Order, error) {
	res := make([]schema.Order, 0)
	err := w.Db.Model(&schema.Order{}).Where("payment_status = ?  and on_chain_status = ? and sort = ?", schema.SuccPayment, schema.WaitOnChain, false).Limit(2000).Find(&res).Error
	return res, err
}

func (w *Database) GetNeedOnChainOrdersSorted() ([]schema.Order, error) {
	res := make([]schema.Order, 0)
	err := w.Db.Model(&schema.Order{}).Where("payment_status = ?  and on_chain_status = ? and sort = ?", schema.SuccPayment, schema.WaitOnChain, true).Limit(2000).Find(&res).Error
	return res, err
}

func (w *Database) UpdateOrdOnChainStatus(itemId, status string, tx *gorm.DB) error {
	db := w.Db
	if tx != nil {
		db = tx
	}
	return db.Model(&schema.Order{}).Where("item_id = ?", itemId).Update("on_chain_status", status).Error
}

func (w *Database) GetOrdersBySigner(signer string, cursorId int64, num int) ([]schema.Order, error) {
	if cursorId <= 0 {
		cursorId = math.MaxInt64
	}
	records := make([]schema.Order, 0, num)
	err := w.Db.Model(&schema.Order{}).Where("id < ? and signer = ? and on_chain_status != ?", cursorId, signer, schema.FailedOnChain).Order("id DESC").Limit(num).Find(&records).Error
	return records, err
}

func (w *Database) GetOrdersByApiKey(apiKey string, cursorId int64, pageSize int, sort string) ([]schema.Order, error) {
	records := make([]schema.Order, 0, pageSize)
	var err error
	if strings.ToUpper(sort) == "ASC" {
		if cursorId <= 0 {
			cursorId = 0
		}
		err = w.Db.Model(&schema.Order{}).Where("api_key = ? and id > ?", apiKey, cursorId).Order("id ASC").Limit(pageSize).Find(&records).Error
	} else {
		if cursorId <= 0 {
			cursorId = math.MaxInt64
		}
		err = w.Db.Model(&schema.Order{}).Where("api_key = ? and id < ?", apiKey, cursorId).Order("id DESC").Limit(pageSize).Find(&records).Error
	}
	return records, err
}

func (db *Database) ExistProcessedOrderItem(itemId string) (res schema.Order, exist bool) {
	err := db.Db.Model(&schema.Order{}).Where("item_id = ? and (on_chain_status = ? or on_chain_status = ?)", itemId, schema.PendingOnChain, schema.SuccOnChain).First(&res).Error
	if err == nil {
		exist = true
	}
	return
}

func (w *Database) InsertPrices(tps []schema.TokenPrice) error {
	return w.Db.Clauses(clause.OnConflict{DoNothing: true}).Create(&tps).Error
}

func (w *Database) UpdatePrice(symbol string, newPrice float64) error {
	return w.Db.Model(&schema.TokenPrice{}).Where("symbol = ?", symbol).Update("price", newPrice).Error
}

func (w *Database) GetPrices() ([]schema.TokenPrice, error) {
	res := make([]schema.TokenPrice, 0, 10)
	err := w.Db.Find(&res).Error
	return res, err
}

func (w *Database) GetArPrice() (float64, error) {
	res := schema.TokenPrice{}
	err := w.Db.Where("symbol = ?", "AR").First(&res).Error
	return res.Price, err
}

func (w *Database) InsertReceiptTx(tx schema.ReceiptEverTx) error {
	return w.Db.Clauses(clause.OnConflict{DoNothing: true}).Create(&tx).Error
}

func (w *Database) GetLastEverRawId() (uint64, error) {
	tx := schema.ReceiptEverTx{}
	err := w.Db.Model(&schema.ReceiptEverTx{}).Last(&tx).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return tx.RawId, err
}

func (w *Database) GetReceiptsByStatus(status string) ([]schema.ReceiptEverTx, error) {
	res := make([]schema.ReceiptEverTx, 0)
	timestamp := time.Now().UnixMilli() - 24*60*60*1000 // latest 1 day
	err := w.Db.Model(&schema.ReceiptEverTx{}).Where("status = ? and nonce > ?", status, timestamp).Find(&res).Error
	return res, err
}

func (w *Database) UpdateReceiptStatus(rawId uint64, status string, tx *gorm.DB) error {
	db := w.Db
	if tx != nil {
		db = tx
	}
	return db.Model(&schema.ReceiptEverTx{}).Where("raw_id = ?", rawId).Update("status", status).Error
}

func (w *Database) UpdateRefundErr(rawId uint64, errMsg string) error {
	data := make(map[string]interface{})
	data["status"] = schema.RefundErr
	data["err_msg"] = errMsg
	return w.Db.Model(&schema.ReceiptEverTx{}).Where("raw_id = ?", rawId).Updates(data).Error
}

func (w *Database) InsertArTx(tx schema.OnChainTx) error {
	return w.Db.Create(&tx).Error
}

func (w *Database) GetArTxByStatus(status string) ([]schema.OnChainTx, error) {
	res := make([]schema.OnChainTx, 0, 10)
	err := w.Db.Model(schema.OnChainTx{}).Where("status = ?", status).Find(&res).Error
	return res, err
}

func (w *Database) UpdateArTxStatus(arId, status string, arTxStatus *types.TxStatus, tx *gorm.DB) error {
	db := w.Db
	if tx != nil {
		db = tx
	}
	data := make(map[string]interface{})
	data["status"] = status
	if arTxStatus != nil {
		data["block_id"] = arTxStatus.BlockIndepHash
		data["block_height"] = arTxStatus.BlockHeight
	}
	return db.Model(&schema.OnChainTx{}).Where("ar_id = ?", arId).Updates(data).Error
}

func (w *Database) UpdateArTx(id uint, arId string, curHeight int64, dataSize, reward string, status string) error {
	data := make(map[string]interface{})
	data["ar_id"] = arId
	data["cur_height"] = curHeight
	data["data_size"] = dataSize
	data["reward"] = reward
	data["status"] = status
	return w.Db.Model(&schema.OnChainTx{}).Where("id = ?", id).Updates(data).Error
}

func (w *Database) GetKafkaOnChains() ([]schema.OnChainTx, error) {
	results := make([]schema.OnChainTx, 0)
	err := w.Db.Model(&schema.OnChainTx{}).Where("block_height > ? and kafka = ? and status = ?", 1188855, false, schema.SuccOnChain).Limit(10).Find(&results).Error
	return results, err
}

func (w *Database) KafkaOnChainDone(id uint) error {
	return w.Db.Model(&schema.OnChainTx{}).Where("id = ?", id).Update("kafka", true).Error
}

func (w *Database) InsertManifest(mf schema.Manifest) error {
	return w.Db.Create(&mf).Error
}

func (w *Database) GetManifestId(mfUrl string) (string, error) {
	res := schema.Manifest{}
	err := w.Db.Model(&schema.Manifest{}).Where("manifest_url = ?", mfUrl).Last(&res).Error
	return res.ManifestId, err
}

func (w *Database) DelManifest(id string) error {
	return w.Db.Where("manifest_id = ?", id).Delete(&schema.Manifest{}).Error
}

func (w *Database) GetOrderRealTimeStatistic() ([]byte, error) {
	var results []schema.Result
	status := []string{"waiting", "pending", "success", "failed"}
	w.Db.Model(&schema.Order{}).Select("on_chain_status as status ,count(1) as totals,sum(size) as total_data_size").Group("on_chain_status").Find(&results)

	for _, s := range status {
		flag := true
		for i := range results {
			if s == results[i].Status {
				flag = false
			}
		}
		if flag {
			results = append(results, schema.Result{Status: s})
		}
	}
	return json.Marshal(results)
}

func (w *Database) GetOrderStatisticByDate(r schema.Range) ([]*schema.DailyStatistic, error) {
	var orderstatistics []schema.OrderStatistic
	start, _ := time.Parse("20060102", r.Start)
	end, _ := time.Parse("20060102", r.End)
	err := w.Db.Model(&schema.OrderStatistic{}).Where("date >= ? and date <= ?", start, end).Order("date").Find(&orderstatistics).Error
	if err != nil {
		return nil, err
	}
	res := make([]*schema.DailyStatistic, 0)
	for i := range orderstatistics {
		date := orderstatistics[i].Date.Format("20060102")
		res = append(res, &schema.DailyStatistic{
			Date: date,
			Result: schema.Result{
				Status:        schema.SuccOnChain,
				Totals:        orderstatistics[i].Totals,
				TotalDataSize: orderstatistics[i].TotalDataSize,
			},
		})
	}
	return res, nil
}

func (w *Database) GetDailyStatisticByDate(r schema.TimeRange) ([]schema.Result, error) {
	var results []schema.Result
	return results, w.Db.Model(&schema.Order{}).Select("count(1) as totals,sum(size) as total_data_size").Where("updated_at >= ? and updated_at < ? and on_chain_status = ?", r.Start, r.End, "success").Group("on_chain_status").Find(&results).Error
}

func (w *Database) WhetherExec(r schema.TimeRange) bool {
	var osc schema.OrderStatistic
	err2 := w.Db.Model(&schema.OrderStatistic{}).Where("date >= ? and date < ?", r.Start, r.End).First(&osc).Error
	return err2 != nil
}
