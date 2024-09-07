package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/store"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
)

// ErrValidationFailed describes an error when the key is not found
var ErrValidationFailed = errors.New("validation failed")

type requestAuthenticate struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type responseAuthenticate struct {
	Token string `json:"token"`
}

type requestGetTransactionsByHashes struct {
	TransactionHashes []string `validate:"required,max=20,dive,len=66,hexadecimal"`
}

type requestGetTransactionsByRLP struct {
	RLPHex string `validate:"required,max=3000,hexadecimal"`
}

type Transaction struct {
	Hash            string      `json:"transactionHash"`
	Status          int         `json:"transactionStatus"`
	BlockHash       string      `json:"blockHash"`
	BlockNumber     *big.Int    `json:"blockNumber"`
	From            string      `json:"from"`
	To              null.String `json:"to"`
	ContractAddress null.String `json:"contractAddress"`
	LogsCount       int         `json:"logsCount"`
	Input           string      `json:"input"`
	Value           string      `json:"value"`
}

type responseGetTransactionsByHashes struct {
	Transactions []*Transaction `json:"transactions"`
}

type responseGetAllTransactions struct {
	Transactions []*Transaction `json:"transactions"`
}

// GetTransactionsByHashes retrieves eth transactions by tx hashes
func (ep *EndPoint) GetTransactionsByHashes(w http.ResponseWriter, r *http.Request) {
	txHashes := r.URL.Query()["transactionHashes"]

	reqParams := requestGetTransactionsByHashes{TransactionHashes: txHashes}

	validate := validator.New()
	err := validate.Struct(reqParams)
	if err != nil {
		log.Errorf("cannot validate transactionHashes query parameter: %v", err)
		writeJSONError(w, http.StatusUnprocessableEntity, ErrValidationFailed)
		return
	}

	res, done := ep.getTransactionsByHashes(w, r, txHashes)
	if done {
		return
	}

	writeJSONResponse(w, http.StatusOK, res)
}

// GetTransactionsByRLP retrieves eth transactions by RLP encoded list of hashes
func (ep *EndPoint) GetTransactionsByRLP(w http.ResponseWriter, r *http.Request) {
	rlpHex := mux.Vars(r)["rlphex"]

	reqParams := requestGetTransactionsByRLP{RLPHex: rlpHex}

	validate := validator.New()
	err := validate.Struct(reqParams)
	if err != nil {
		log.Errorf("cannot validate rlphex url path: %v", err)
		writeJSONError(w, http.StatusUnprocessableEntity, ErrValidationFailed)
		return
	}

	// convert rlp hex to hex hashes
	txHashes, err := decodeRLPToTxHashes(rlpHex)
	if err != nil {
		log.Errorf("cannot decode rlphex url path: %v", err)
		writeJSONError(w, http.StatusUnprocessableEntity, ErrValidationFailed)
		return
	}

	res, done := ep.getTransactionsByHashes(w, r, txHashes)
	if done {
		return
	}

	writeJSONResponse(w, http.StatusOK, res)
}

// GetAllTransactions retrieves all transactions stored in the database
func (ep *EndPoint) GetAllTransactions(w http.ResponseWriter, _ *http.Request) {
	txList, err := ep.ap.GetAllTransactions()
	if err != nil {
		log.Errorf("cannot retrieve all transactions: %v", err)
		writeInternalServerError(w)
		return
	}

	res := responseGetAllTransactions{
		Transactions: []*Transaction{},
	}

	for _, tx := range txList {
		blockNumber := new(big.Int)
		tx.BlockNumber.Int(blockNumber)

		res.Transactions = append(res.Transactions,
			&Transaction{
				Hash:            tx.TXHash,
				Status:          tx.TXStatus,
				BlockHash:       tx.BlockHash,
				BlockNumber:     blockNumber,
				From:            tx.FromAddress,
				To:              tx.ToAddress,
				ContractAddress: tx.ContractAddress,
				LogsCount:       int(tx.LogsCount),
				Input:           tx.Input,
				Value:           tx.Value,
			},
		)
	}

	writeJSONResponse(w, http.StatusOK, res)
}

// GetMyTransactions retrieves "my" transactions stored in the database
func (ep *EndPoint) GetMyTransactions(w http.ResponseWriter, r *http.Request) {
	// extract the user ID, cannot be missing
	userID, _ := r.Context().Value(userIDKey).(int)

	txList, err := ep.ap.GetMyTransactions(userID)
	if err != nil {
		log.Errorf("cannot retrieve my transactions: %v", err)
		writeInternalServerError(w)
		return
	}

	res := responseGetAllTransactions{
		Transactions: []*Transaction{},
	}

	for _, tx := range txList {
		blockNumber := new(big.Int)
		tx.BlockNumber.Int(blockNumber)

		res.Transactions = append(res.Transactions,
			&Transaction{
				Hash:            tx.TXHash,
				Status:          tx.TXStatus,
				BlockHash:       tx.BlockHash,
				BlockNumber:     blockNumber,
				From:            tx.FromAddress,
				To:              tx.ToAddress,
				ContractAddress: tx.ContractAddress,
				LogsCount:       int(tx.LogsCount),
				Input:           tx.Input,
				Value:           tx.Value,
			},
		)
	}

	writeJSONResponse(w, http.StatusOK, res)
}

func (ep *EndPoint) Authenticate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeBadRequestError(w)
		return
	}

	var authRequest requestAuthenticate
	err = json.Unmarshal(body, &authRequest)

	if err != nil {
		writeBadRequestError(w)
		return
	}

	// don't check the credentials, but username and password cannot be empty
	if authRequest.Username == "" || authRequest.Password == "" {
		writeUnauthorizedError(w)
		return
	}

	user, err := ep.ap.GetUser(authRequest.Username, authRequest.Password)
	if err != nil {
		log.Errorf("cannot get user info: %v", err)
		writeInternalServerError(w)
		return
	}

	if user.ID == store.NonAuthenticatedUser {
		writeUnauthorizedError(w)
		return
	}

	// we are authenticated, create the token
	token, err := createToken(ep.vp.GetString(cmd.JWTSecret), user.ID)

	if err != nil {
		writeInternalServerError(w)
		return
	}

	res := responseAuthenticate{Token: token}
	writeJSONResponse(w, http.StatusOK, res)
}

func (ep *EndPoint) getTransactionsByHashes(w http.ResponseWriter, r *http.Request, txHashes []string,
) (responseGetTransactionsByHashes, bool) {
	// unify the hash format
	for i := range txHashes {
		txHashes[i] = strings.ToLower(txHashes[i])
	}

	// extract the user ID - zero value for "no user"
	userID, _ := r.Context().Value(userIDKey).(int)

	txList, err := ep.ap.GetTransactionsByHashes(r.Context(), txHashes, userID)
	if err != nil {
		log.Errorf("cannot retrieve transactions by hashes: %v", err)
		writeInternalServerError(w)
		return responseGetTransactionsByHashes{}, true
	}

	res := responseGetTransactionsByHashes{
		Transactions: []*Transaction{},
	}

	for _, tx := range txList {
		blockNumber := new(big.Int)
		tx.BlockNumber.Int(blockNumber)

		res.Transactions = append(res.Transactions,
			&Transaction{
				Hash:            tx.TXHash,
				Status:          tx.TXStatus,
				BlockHash:       tx.BlockHash,
				BlockNumber:     blockNumber,
				From:            tx.FromAddress,
				To:              tx.ToAddress,
				ContractAddress: tx.ContractAddress,
				LogsCount:       int(tx.LogsCount),
				Input:           tx.Input,
				Value:           tx.Value,
			},
		)
	}
	return res, false
}

func createToken(jwtSecret string, userID int) (string, error) {
	iat := time.Now()
	exp := iat.Add(4 * time.Hour)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "https://limechain.tech/auth",
		"exp": exp.Unix(),
		"iat": iat.Unix(),
		"sub": userID,
	})

	// sign and get the encoded token as a string using the secret
	return token.SignedString([]byte(jwtSecret))
}

func decodeRLPToTxHashes(rlpHex string) ([]string, error) {
	// decode the {rlphex} path parameter to bytes
	rlpBytes, err := hex.DecodeString(rlpHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %v", err)
	}

	// decode the RLP bytes to string slice
	var txHashes []string
	err = rlp.DecodeBytes(rlpBytes, &txHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode RLP: %v", err)
	}

	return txHashes, nil
}
