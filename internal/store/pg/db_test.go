package pg

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/store"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/ericlagergren/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/types"
	"golang.org/x/crypto/bcrypt"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/stretchr/testify/suite"
)

// This test suite is added to satisfy the following requirement:
//
// "Create a unit test suite that proves that:
//   - Your database can store and retrieve data"
//
// In order to do that, we are using DB transaction Begin/Rollback testing strategy with the actual database.
// Each test from the suite creates DB transaction in the beginning and ends with Rollback.
// Note: DB_CONNECTION_URL must be provided to run that suite, or the tests will be skipped.
type StorageTestSuite struct {
	suite.Suite
	ctx context.Context
	st  store.StorageProvider
	db  boil.Executor
}

// this function executes before the test suite begins execution
func (s *StorageTestSuite) SetupSuite() {
	vp := cmd.NewViper()

	if vp.GetString(cmd.DBConnectionURL) == "" {
		s.T().Skip("Skipping Storage test suite, since the env variable DB_CONNECTION_URL is not provided!")
	}

	cmd.LogInit("fatal")
	ctx := context.Background()
	st, err := NewStore(ctx, vp)
	if err != nil {
		log.Fatalf("cannot initialize database: %v", err)
	}

	s.ctx = ctx
	s.st = st
	// backup the original db handler to have "fresh", non-tx version available for each test
	s.db = boil.GetDB()
}

// this function executes before each test case
func (s *StorageTestSuite) SetupTest() {
	boil.SetDB(s.db)
	dbTx, err := s.st.(*Store).BeginTx()
	if err != nil {
		s.FailNow("cannot start tx")
	}
	boil.SetDB(dbTx)
}

// this function executes after each test case
func (s *StorageTestSuite) TearDownTest() {
	tx := boil.GetDB().(*sql.Tx)
	err := s.st.(*Store).RollbackTx(tx)
	if err != nil {
		s.FailNow("cannot rollback tx")
	}
}

func (s *StorageTestSuite) TestGetTransactionsByHashes() {
	txList := mockEthereumTransactions()

	r := s.Require()

	user := mockUser(-1)

	// create test user -1
	err := user.Insert(s.ctx, boil.GetContextDB(), boil.Infer())
	r.Nil(err, "fail to insert user")

	// insert couple ethereum transactions under that user
	err = s.st.InsertTransactions(txList, user.ID)
	r.Nil(err, "fail to insert transactions")

	// check whether those transactions are available under that user
	myList, err := s.st.GetTransactionsByHashes([]string{txList[0].TXHash, txList[1].TXHash}, user.ID)
	r.Nil(err, "fail to get my transactions for the user")

	foundCnt := containsTransactions(myList, txList)
	r.Equal(foundCnt, len(txList), "transactions cannot be found")
}

func (s *StorageTestSuite) TestGetAllTransactions() {
	txList := mockEthereumTransactions()

	r := s.Require()

	user := mockUser(-1)

	// create test user
	err := user.Insert(s.ctx, boil.GetContextDB(), boil.Infer())
	r.Nil(err, "fail to insert user")

	// and insert couple ethereum transactions under that user
	err = s.st.InsertTransactions(txList, user.ID)
	r.Nil(err, "fail to insert transactions")

	// now get all transactions independently of any user
	allList, err := s.st.GetAllTransactions()
	r.Nil(err, "fail to get all transactions")

	foundCnt := containsTransactions(allList, txList)

	r.Equal(foundCnt, len(txList), "transactions cannot be found")
}

func (s *StorageTestSuite) TestGetMyTransactions() {
	txList := mockEthereumTransactions()

	r := s.Require()

	user := mockUser(-5)

	// create test user
	err := user.Insert(s.ctx, boil.GetContextDB(), boil.Infer())
	r.Nil(err, "fail to insert user")

	// and insert couple tx under that user
	err = s.st.InsertTransactions(txList, user.ID)
	r.Nil(err, "fail to insert transactions")

	// now fetch only those txs that are "mine"
	allList, err := s.st.GetMyTransactions(user.ID)
	r.Nil(err, "fail to get my transactions for the user")

	foundCnt := containsTransactions(allList, txList)

	r.Equal(foundCnt, len(txList), "transactions cannot be found")
}

func (s *StorageTestSuite) TestGetUser() {
	r := s.Require()

	user := mockUser(-2)

	// plain text user password
	pwd := user.Password

	// hash the password with salt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	r.Nilf(err, "fail generating bcrypt hash: %v", err)
	user.Password = string(hashedPassword)

	// insert the user with hashed password
	err = user.Insert(s.ctx, boil.GetContextDB(), boil.Infer())
	r.Nil(err, "fail to insert user")

	// get the user by provided plain text username ana password
	freshUser, err := s.st.GetUser(user.Username, pwd)
	r.Nil(err, "fail to get the user")

	r.Equal(freshUser.ID, user.ID, "user cannot be found")
}

func (s *StorageTestSuite) TestInsertTransactionsUser() {
	txList := mockEthereumTransactions()

	r := s.Require()

	user := mockUser(-3)

	// insert the user
	err := user.Insert(s.ctx, boil.GetContextDB(), boil.Infer())
	r.Nil(err, "fail to insert user")

	// store the ethereum transactions without "attaching" user to them
	err = s.st.InsertTransactions(txList, store.NonAuthenticatedUser)
	r.Nil(err, "fail to insert transactions")

	// verify that NO user_transactions records were created
	myList, err := s.st.GetMyTransactions(user.ID)
	r.Nil(err, "fail to get my transactions for the user")

	foundCnt := containsTransactions(myList, txList)
	r.Equal(foundCnt, 0, "unexpected transactions were found")

	// later, add the respective records to the join table
	err = s.st.InsertTransactionsUser(txList, user.ID)
	r.Nil(err, "fail to insert user_transactions records")

	// verify that those records are there (they should appear as "my" txs)
	myList, err = s.st.GetMyTransactions(user.ID)
	r.Nil(err, "fail to get my transactions for the user")

	foundCnt = containsTransactions(myList, txList)
	r.Equal(foundCnt, len(txList), "transactions cannot be found")
}

func TestStorageTestSuite(t *testing.T) {
	suite.Run(t, new(StorageTestSuite))
}

func containsTransactions(allList, txList []*models.Transaction) int {
	// find all the inserted transactions
	foundCnt := 0
	slices.ContainsFunc(allList, func(tx *models.Transaction) bool {
		for _, v := range txList {
			if tx.TXHash == v.TXHash {
				foundCnt++
			}
		}
		return false
	})
	return foundCnt
}

func mockUser(userID int) *models.User {
	user := &models.User{
		ID:       userID,
		Username: gofakeit.Username(),
		Password: gofakeit.Password(true, true, true, false, false, 10),
	}
	return user
}

func mockEthereumTransactions() []*models.Transaction {
	txList := []*models.Transaction{{
		TXHash:    "0x11113f7adff7fbfc2a10b22a6710331ee68f2e4d1cd73a584d57c8821df71111",
		TXStatus:  1,
		BlockHash: "0x61914f9b5d11dcf30b943f9b6adf4d1c965f31de9157094ec2c51714cb505577",
		BlockNumber: types.Decimal{
			Big: decimal.New(5703601, 0),
		},
		FromAddress:     "0x1fc35B79FB11Ea7D4532dA128DfA9Db573C51b09",
		ToAddress:       null.StringFrom("0xAa449E0226B45D2044B1f721D04001fDe02ABb08"),
		ContractAddress: null.String{},
		LogsCount:       0,
		Input:           "0x",
		Value:           "500000000000000000",
	},
		{
			TXHash:    "0x22223f7adff7fbfc2a10b22a6710331ee68f2e4d1cd73a584d57c8821df72222",
			TXStatus:  1,
			BlockHash: "0xc5a3664f031da2458646a01e18e6957fd1f43715524d94b7336a004b5635837d",
			BlockNumber: types.Decimal{
				Big: decimal.New(5702816, 0),
			},
			FromAddress:     "0xd5e6f34bBd4251195c03e7Bf3660677Ed2315f70",
			ToAddress:       null.StringFrom("0x4c16D8C078eF6B56700C1BE19a336915962df072"),
			ContractAddress: null.String{},
			LogsCount:       1,
			Input:           "0x6a627842000000000000000000000000d5e6f34bbd4251195c03e7bf3660677ed2315f70",
			Value:           "0",
		},
	}
	return txList
}
