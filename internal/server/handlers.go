package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"time"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
	TransactionHashes []string `form:"transactionHashes" validate:"required,dive,len=66,hexadecimal"`
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

// GetTransactionsByHashes retrieves eth transactions by tx hashes
func (ep *EndPoint) GetTransactionsByHashes(w http.ResponseWriter, r *http.Request) {
	txHashes := r.URL.Query()["transactionHashes"]

	reqParams := requestGetTransactionsByHashes{TransactionHashes: txHashes}

	// Validate the parameters
	validate := validator.New()
	err := validate.Struct(reqParams)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, ErrValidationFailed)
		return
	}

	// extract the user ID - zero value for "no user"
	userID, _ := r.Context().Value(userIDKey).(int)

	txList, err := ep.ap.GetTransactionsByHashes(txHashes, userID)
	if err != nil {
		log.Errorf("cannot retrieve transactions by hashes: %v", err)
		writeInternalServerError(w)
		return
	}

	res := responseGetTransactionsByHashes{
		Transactions: []*Transaction{},
	}

	for _, tx := range txList {
		//blockNumber, _ := tx.BlockNumber.Int64()
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

// GetTransactionsByRLP retrieves eth transactions by tx hashes
func (ep *EndPoint) GetTransactionsByRLP(w http.ResponseWriter, r *http.Request) {
	txHashes := r.URL.Query()["transactionHashes"]

	reqParams := requestGetTransactionsByHashes{TransactionHashes: txHashes}

	// Validate the parameters
	validate := validator.New()
	err := validate.Struct(reqParams)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, ErrValidationFailed)
		return
	}

	res := responseGetTransactionsByHashes{
		Transactions: []*Transaction{
			{
				Hash:        "0x017e74eefee2118098a44541bba086b1a162d5f9fd8ef629733b23a56a0ef7e0",
				Status:      0,
				BlockHash:   "",
				BlockNumber: new(big.Int),
				From:        "",
				To: null.String{
					String: "",
					Valid:  true,
				},
				ContractAddress: null.String{
					String: "",
					Valid:  true,
				},
				LogsCount: 0,
				Input:     "",
				Value:     "0",
			},
		},
	}

	writeJSONResponse(w, http.StatusOK, res)
}

// GetAllTransactions retrieves all transactions store in the database
func (ep *EndPoint) GetAllTransactions(w http.ResponseWriter, r *http.Request) {
	txHashes := r.URL.Query()["transactionHashes"]

	reqParams := requestGetTransactionsByHashes{TransactionHashes: txHashes}

	// Validate the parameters
	validate := validator.New()
	err := validate.Struct(reqParams)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, ErrValidationFailed)
		return
	}

	res := responseGetTransactionsByHashes{
		Transactions: []*Transaction{
			{
				Hash:        "0x017e74eefee2118098a44541bba086b1a162d5f9fd8ef629733b23a56a0ef7e0",
				Status:      0,
				BlockHash:   "",
				BlockNumber: new(big.Int),
				From:        "",
				To: null.String{
					String: "",
					Valid:  true,
				},
				ContractAddress: null.String{
					String: "",
					Valid:  true,
				},
				LogsCount: 0,
				Input:     "",
				Value:     "0",
			},
		},
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

	user, err := models.Users(
		qm.Select(models.UserColumns.ID),
		qm.Where(models.UserColumns.Username+"=?", authRequest.Username),
		qm.And(models.UserColumns.Password+"= crypt(?, "+models.UserColumns.Password+")", authRequest.Password),
	).One(ep.ctx, boil.GetContextDB())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeUnauthorizedError(w)
			return
		}
		log.Errorf("cannot read users table: %v", err)
		writeInternalServerError(w)
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
