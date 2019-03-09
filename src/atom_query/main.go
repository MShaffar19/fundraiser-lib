package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Prefunder details
var (
	EarlyDonorsUSD = 300000

	PreContribAtoms float64 = 16856718
	PreContribUSD   float64 = 1329472

	FoundersInflate = 1.33333333
)

// Price info
var (
	AtomPerBtc float64 = 11635
	AtomPerEth float64 = 452.30
	AtomPerUsd float64 = 10

	MaxAtoms = 1000000 * AtomPerUsd
)

// Flags
var (
	ListeningPort int
	DonationsJSON string

	StatsFlag     bool
	ListFlag      bool
	CSVFlag       bool
	BadFlag       bool
	OverLimitFlag bool
	BuildFlag     bool
)

// Database
var (
	atomsMap      map[string]TotalDonationInfo = make(map[string]TotalDonationInfo)
	lateDonations []RawDonationInfo
)

type TotalDonationInfo struct {
	BTC  CoinDonationInfo `json:"btc"` // total btc donated
	ETH  CoinDonationInfo `json:"eth"` // total eth donated
	Atom float64          `json"atom"` // total atom suggested
}

type CoinDonationInfo struct {
	Num   int     `json:"num"`   // number of donations
	Total float64 `json:"total"` // total donated in this coin
	Atom  float64 `json:"atom"`  // suggested atom for this coin
}

// Raw JSON data
type RawDonationInfo struct {
	Type        string  `json:"type"`
	TxID        string  `json:"txid"`
	Address     string  `json:"address"`
	Amount      float64 `json:"amount"`
	Atoms       float64 `json:"atoms"`
	Error       string  `json:"error"`
	BlockHeight int     `json:"blockHeight"`
}

// read a list of raw donations
// and update the atomsMap
func loadDonations(f string) error {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}
	var donationsArray []RawDonationInfo
	if err := json.Unmarshal(b, &donationsArray); err != nil {
		return err
	}

	for _, d := range donationsArray {
		if d.Error != "" {
			if d.BlockHeight == 460661 {
				// is good
			} else if d.Error == "Block too late" {
				lateDonations = append(lateDonations, d)
			} else {
				continue
			}
		}

		totalDonationInfo := atomsMap[d.Address]

		switch d.Type {
		case "btc":
			totalDonationInfo.BTC.Num += 1
			totalDonationInfo.BTC.Total += d.Amount
			totalDonationInfo.BTC.Atom += d.Amount * AtomPerBtc
		case "eth":
			totalDonationInfo.ETH.Num += 1
			totalDonationInfo.ETH.Total += d.Amount
			totalDonationInfo.ETH.Atom += float64(int64(d.Amount*AtomPerEth + 0.00000001))
		default:
			panic(fmt.Sprintf("%v", d))
		}

		atomsMap[d.Address] = totalDonationInfo
	}

	// refund whale
	x := atomsMap["aff9f5a716cdd701304eae6fc7f42c80fdeea584"]
	x.ETH.Total -= (809200 / 45.23)
	x.ETH.Atom -= 8092000
	atomsMap["aff9f5a716cdd701304eae6fc7f42c80fdeea584"] = x

	// Round atoms
	for addr, info := range atomsMap {
		info.BTC.Atom = round2(info.BTC.Atom)
		info.ETH.Atom = info.ETH.Atom // is already whole amount.
		info.Atom = info.BTC.Atom + info.ETH.Atom
		atomsMap[addr] = info

		// strip trailing 0's
		atomAmount := fmt.Sprintf("%.2f", info.Atom)
		if atomAmount[len(atomAmount)-1] == '0' {
			atomAmount = atomAmount[:len(atomAmount)-1]
			if atomAmount[len(atomAmount)-1] == '0' {
				atomAmount = atomAmount[:len(atomAmount)-2]
			}
		}
		fmt.Printf("\"%v\": %v,\n", addr, atomAmount)
	}

	return nil
}

// For sorting balances
type AtomBalances []Account

func (ab AtomBalances) Len() int           { return len(ab) }
func (ab AtomBalances) Less(i, j int) bool { return ab[i].Amount > ab[j].Amount }
func (ab AtomBalances) Swap(i, j int) {
	x := ab[j]
	ab[j] = ab[i]
	ab[i] = x
}

type Account struct {
	Address string  `json:"address"`
	Amount  float64 `json:"balance"`
}

func main() {
	flag.IntVar(&ListeningPort, "port", 8080, "listening port")
	flag.StringVar(&DonationsJSON, "donations", "data/donations.json", "file containing json list of all donations")
	flag.BoolVar(&StatsFlag, "stats", false, "dump the stats and exit")
	flag.BoolVar(&ListFlag, "list", false, "list the sorted addresses and exit")
	flag.BoolVar(&CSVFlag, "csv", false, "list the sorted percentage atoms in csv")
	flag.BoolVar(&BadFlag, "bad", false, "list txs that were late")
	flag.BoolVar(&OverLimitFlag, "limit", false, "list accounts that went over the limit")
	flag.BoolVar(&BuildFlag, "build", false, "output preliminary fundraiser_atoms.json not including prefunders")
	flag.Parse()

	if err := loadDonations(DonationsJSON); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if StatsFlag {
		printStats()
	} else if ListFlag {
		printList()
	} else if CSVFlag {
		printCSV()
	} else if BadFlag {
		printBadTxs()
	} else if OverLimitFlag {
		printOverLimit()
	} else if BuildFlag {
		printBuild()
	} else {
		fmt.Println("Listening on port", ListeningPort)
		http.HandleFunc("/atoms/", queryAtoms)
		http.ListenAndServe(fmt.Sprintf(":%d", ListeningPort), nil)
	}

}

func sumAccounts() (totalEth, totalBtc, totalAtom float64, accounts AtomBalances) {
	totalEthAtom, totalBtcAtom := float64(0), float64(0)
	for addr, info := range atomsMap {
		totalBtc += info.BTC.Total
		totalBtcAtom += info.BTC.Atom
		totalEth += info.ETH.Total
		totalEthAtom += info.ETH.Atom
		totalAtom += info.Atom
		accounts = append(accounts, Account{addr, info.Atom})
	}
	fmt.Println("totalAtom", totalAtom)
	fmt.Println("totalBtcAtom", totalBtcAtom)
	fmt.Println("totalEthAtom", totalEthAtom)

	sort.Sort(accounts)

	totalAtom += PreContribAtoms
	totalAtom *= FoundersInflate

	return totalEth, totalBtc, totalAtom, accounts
}

func printBadTxs() {
	var sum float64
	for _, info := range lateDonations {
		sum += info.Amount
		fmt.Println(info.Address, info.TxID, info.BlockHeight, info.Amount)
	}
	fmt.Println("")
	fmt.Println("Total BTC", sum)
}

func printOverLimit() {
	_, _, _, accounts := sumAccounts()
	for _, acc := range accounts {
		if acc.Amount > MaxAtoms {
			di := atomsMap[acc.Address]
			s := fmt.Sprintf("%s", acc.Address)
			if di.BTC.Total > 0 {
				s += fmt.Sprintf("\t BTC: $%f", di.BTC.Total*AtomPerBtc/AtomPerUsd)
			}
			if di.ETH.Total > 0 {
				s += fmt.Sprintf("\t ETH: $%f", di.ETH.Total*AtomPerEth/AtomPerUsd)
			}
			fmt.Println(s)
		}
	}
}

func printList() {
	_, _, totalAtom, accounts := sumAccounts()
	for i, acc := range accounts {
		fmt.Println(i, acc.Address, 100*acc.Amount/totalAtom)
	}
}

func printBuild() {
	_, _, _, accounts := sumAccounts()
	b, err := json.MarshalIndent(accounts, "", "\t")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

func printCSV() {
	_, _, totalAtom, accounts := sumAccounts()
	s := ""
	for _, acc := range accounts {
		s += fmt.Sprintf("%f,", 100*acc.Amount/totalAtom)
	}
	s = s[:len(s)-1]
	fmt.Println(s)
}

func printStats() {
	totalEth, totalBtc, totalAtom, accounts := sumAccounts()

	var top float64
	for i := 0; i < 10; i++ {
		top += accounts[i].Amount
	}
	fmt.Println("Top 10:", 100*top/totalAtom)

	fmt.Println("")
	fmt.Println("Total Unique Donations", len(atomsMap))
	fmt.Println("Total BTC", totalBtc)
	fmt.Println("Total ETH", totalEth)
	fmt.Println("Total ATOM", totalAtom)
}

func queryAtoms(w http.ResponseWriter, r *http.Request) {
	spl := strings.Split(r.URL.Path[1:], "/")
	if len(spl) != 2 {
		httpError(w, "Invalid query")
		return
	}

	addr := spl[1]
	if len(addr) != 42 && len(addr) != 40 {
		httpError(w, "Invalid addr length")
		return
	}

	addr = strings.TrimPrefix(addr, "0x")
	_, err := hex.DecodeString(addr)
	if err != nil {
		httpError(w, err.Error())
		return
	}

	info := atomsMap[addr]
	infoBytes, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		httpError(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8600")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Content-Length, Cache-Control, cf-connecting-ip")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Write(infoBytes)
}

func httpError(w http.ResponseWriter, errStr string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(errStr + "\n"))
}

// round to 2 decimal places
func round2(x float64) (r float64) {
	s := fmt.Sprintf("%.2f", x)
	r, _ = strconv.ParseFloat(s, 64)
	return
}
