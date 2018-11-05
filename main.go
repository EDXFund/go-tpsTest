package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/miguelmota/go-ethereum-hdwallet"
	"log"
	"math"
	"math/big"
	"strconv"
	"sync"
	"time"
)

type testAccount struct {
	account accounts.Account
	index   int
	balance *big.Int
	nonce   uint64
	curTx   *common.Hash
	locker  sync.RWMutex
}

const power = 2

//const rawUrl string = "wss://ropsten.infura.io/ws"
const rawUrl string = "ws://127.0.0.1:8545"
const mnemonic string = "whip matter defense behave advance boat belt purse oil hamster stable clump"
const checkSecends = 5

func main() {

	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		log.Fatal(err)
	}

	client, err := ethclient.Dial(rawUrl)
	if err != nil {
		log.Fatalln(err)
	}

	walletLen := uint(math.Pow(2, float64(power)))
	wallets := make([]*testAccount, walletLen, walletLen)

	//var wallets [uint(math.Pow(2,float64(power)))]*testAccount

	//init wallets
	var wg sync.WaitGroup

	for i := range wallets {

		wg.Add(1)
		a := i
		go func() {
			defer wg.Done()
			path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/" + strconv.Itoa(a))
			account, err := wallet.Derive(path, true)
			if err != nil {
				log.Fatal(err)
			}

			wallets[a] = &testAccount{
				account: account,
				index:   a,
				balance: nil,
				nonce:   uint64(0),
				curTx:   nil,
			}

			balance, err := client.BalanceAt(context.Background(), account.Address, nil)
			wallets[a].balance = balance

			nonce, err := client.NonceAt(context.Background(), account.Address, nil)

			wallets[a].nonce = nonce
			fmt.Printf("ok: %v\n", wallets[a].balance)
		}()

		fmt.Printf("start: %d\n", a)

	}

	wg.Wait()

	//按顺序执行
	for i := power; i > 0; i-- {
		aLen := uint64(math.Pow(2, float64(power-i)))
		gasPrice, _ := client.SuggestGasPrice(context.Background())
		var wg sync.WaitGroup
		for x := aLen; x > 0; x-- {
			from := x - 1
			to := x + aLen - 1

			wg.Add(1)
			go func() {
				walletFrom := wallets[from]
				walletTo := wallets[to]

				cmp := walletFrom.balance.Cmp(big.NewInt(0).Mul(gasPrice, big.NewInt(21000)))

				if cmp <= 0 {
					return
				}

				nonce := walletFrom.nonce
				value := GetAverageValue(walletFrom.balance, gasPrice)
				gasLimit := uint64(21000)
				toAddress := walletTo.account.Address

				var data []byte
				tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

				//privateKey,_ := wallet.PrivateKey(wallets[0].account)
				//signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, privateKey)
				signedTx, _ := wallet.SignTx(walletFrom.account, tx, nil)
				hash := signedTx.Hash()
				walletFrom.curTx = &hash

				for {
					err := client.SendTransaction(context.Background(), signedTx)
					if err != nil {
						continue
					} else {
						break
					}
				}

				for {
					time.Sleep(time.Second * checkSecends)
					if (walletFrom.curTx == nil || *walletFrom.curTx == common.Hash{}) {

					} else {
						_, state, err := client.TransactionByHash(context.Background(), *walletFrom.curTx)

						if state && err == nil {
							walletFrom.curTx = nil
							balance, err := client.BalanceAt(context.Background(), walletFrom.account.Address, nil)
							if err != nil {
								continue
							}
							walletFrom.balance = balance
							walletTo.balance = balance
							nonce, _ := client.NonceAt(context.Background(), walletFrom.account.Address, nil)
							walletFrom.nonce = nonce

							nonce2, _ := client.NonceAt(context.Background(), walletTo.account.Address, nil)
							walletTo.nonce = nonce2

							wg.Done()
							break
						}
					}
				}

			}()

			fmt.Printf("tran: %d --> %d \n", from, to)

			//endSin := make(chan interface{})

		}

		wg.Wait()

	}

}

func (account *testAccount) checkTx(client *ethclient.Client, wg *sync.WaitGroup) {
	for {
		time.Sleep(time.Second * checkSecends)
		if (account.curTx == nil || *account.curTx == common.Hash{}) {
			wg.Done()
			return
		} else {
			_, state, _ := client.TransactionByHash(context.Background(), *account.curTx)

			if state || !state {
				account.curTx = nil
				balance, _ := client.BalanceAt(context.Background(), account.account.Address, nil)
				account.balance = balance

				nonce, _ := client.NonceAt(context.Background(), account.account.Address, nil)

				account.nonce = nonce
				wg.Done()
				return
			}
		}
	}

}

func GetAverageValue(a *big.Int, gasPrice *big.Int) *big.Int {
	fgasFee := big.NewInt(0).Mul(gasPrice, big.NewInt(-21000))
	total := big.NewInt(0).Add(a, fgasFee)
	div := big.NewInt(0).Add(a, fgasFee).Div(total, big.NewInt(2))
	return div
}

func GetMaxValue(a *big.Int, gasPrice *big.Int) *big.Int {
	fgasFee := big.NewInt(0).Mul(gasPrice, big.NewInt(-21000))
	total := a.Add(a, fgasFee)
	return total
}
