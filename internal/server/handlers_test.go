package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/ericlagergren/decimal"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/types"

	servicemocks "ethereum-fetcher/internal/app/mocks"
)

// This test suite is added to satisfy the following requirement:
//
// "Create a unit test suite that proves that:
//
//   - Your RLP decoding works
//   - You are generating and decoding JWT tokens correctly"
//
// In order to do that, we are using mocks for both storage (database) and network (ethereum node)
type EndpointTestSuite struct {
	suite.Suite
	vp  *viper.Viper
	ctx context.Context
}

// this function executes before the test suite begins execution
func (s *EndpointTestSuite) SetupSuite() {
	vp := cmd.NewViper()
	vp.SetDefault(cmd.JWTSecret, "testJWTSecret")
	s.vp = vp

	cmd.LogInit("fatal")
}

// this function executes before each test case
func (s *EndpointTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *EndpointTestSuite) TestGetTransactionsByRLPEndpoints() {
	t := s.T()

	type expected struct {
		statusCode int
		txHashes   []string
		err        error
	}

	tests := []struct {
		name string
		exp  expected
		args string
	}{
		{
			name: "with provided valid rlp encoded list, it returns OK",
			exp: expected{statusCode: http.StatusOK, err: nil,
				txHashes: []string{
					"0x307866633262336236646233386135316462336239636239356465323962373139646538646562393936333036323665346234623939646630353666666237663265",
					"0x307834383630336637616466663766626663326131306232326136373130333331656536386632653464316364373361353834643537633838323164663739333536",
					"0x307863626339323065376262383963626362353430613436396131363232366266313035373832353238336162386561633366343564303038313165656638613634",
					"0x307836643630346666633634346132383266636138636238653737386531653366383234356438626431643439333236653330313661336338373862613063626264",
				},
			},
			args: `f90110b842307866633262336236646233386135316462336239636239356465323962373139646538646562393936333036323665346234623939646630353666666237663265b842307834383630336637616466663766626663326131306232326136373130333331656536386632653464316364373361353834643537633838323164663739333536b842307863626339323065376262383963626362353430613436396131363232366266313035373832353238336162386561633366343564303038313165656638613634b842307836643630346666633634346132383266636138636238653737386531653366383234356438626431643439333236653330313661336338373862613063626264`,
		},
		{
			name: "with provided valid rlp encoded list of single element, it returns OK",
			exp:  expected{statusCode: http.StatusOK, err: nil},
			args: `f844b842307866633262336236646233386135316462336239636239356465323962373139646538646562393936333036323665346234623939646630353666666237663265`,
		},
		{
			name: "with provided broken rlp encoded list, it returns UnprocessableEntity",
			exp: expected{statusCode: http.StatusUnprocessableEntity, err: nil,
				txHashes: []string{},
			},
			args: `316462336239636239356465323962373139646538646562393936333036323665346234623939646630353666666237663265`,
		},
		{
			name: "with provided single string instead of list, it returns UnprocessableEntity",
			exp: expected{statusCode: http.StatusUnprocessableEntity, err: nil,
				txHashes: []string{},
			},
			args: `b842307866633262336236646233386135316462336239636239356465323962373139646538646562393936333036323665346234623939646630353666666237663265`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "http://127.0.0.1/lime/eth/"+tt.args, bytes.NewBufferString(""))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			app := servicemocks.NewServiceProvider(s.T())

			var txList []*models.Transaction

			if tt.exp.txHashes != nil {
				txList = mockSetupTransactions(tt.exp.txHashes)
			}
			app.On("GetTransactionsByHashes", mock.AnythingOfType("[]string"), mock.AnythingOfType("int")).
				Return(txList, tt.exp.err).Maybe()

			ep := NewEndPoint(s.ctx, s.vp, app)

			// prepare the /lime/eth/{rlphex} endpoint and call it
			r := mux.NewRouter()
			r.HandleFunc("/lime/eth/{rlphex}", ep.GetTransactionsByRLP)
			r.ServeHTTP(response, request)

			require.Equal(t, tt.exp.statusCode, response.Code)

			if tt.exp.statusCode == http.StatusOK {
				require.NotEmpty(t, response.Body.String())

				// extract returned transactions
				resp := new(responseGetTransactionsByHashes)
				err := json.Unmarshal(response.Body.Bytes(), resp)
				require.NoError(t, err)

				require.Len(t, resp.Transactions, len(txList))

				// compare decoded RLPs to expected list
				for i := range resp.Transactions {
					require.Equal(t, txList[i].TXHash, resp.Transactions[i].Hash)
				}
			}
		})
	}
}

func (s *EndpointTestSuite) TestAuthenticateEndpoints() {
	t := s.T()

	bobUser := &models.User{
		ID:       2,
		Username: "bob",
		Password: "bob",
	}

	noUser := &models.User{
		ID: 0,
	}

	type expected struct {
		statusCode int
		user       *models.User
		token      string
		err        error
	}

	tests := []struct {
		name string
		exp  expected
		args string
	}{
		{
			name: "with provided valid username and password, it returns OK and JWT",
			exp:  expected{statusCode: http.StatusOK, user: bobUser, err: nil},
			args: `{"username": "bob", "password": "bob"}`,
		},
		{
			name: "with provided incorrect username and password, it returns Unauthorized",
			exp:  expected{statusCode: http.StatusUnauthorized, user: noUser, err: nil},
			args: `{"username": "bob", "password": "mob"}`,
		},
		{
			name: "with provided valid username and password, it returns OK, with replaced and expired JWT, validation returns Unauthorized",
			exp: expected{statusCode: http.StatusOK, user: bobUser, err: nil,
				token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjU0NzIzNjUsImlhdCI6MTcyNTQ4Njc2NSwiaXNzIjoiaHR0cHM6Ly9saW1lY2hhaW4udGVjaC9hdXRoIiwic3ViIjoyfQ.PTCRLJpR3Pr6p9jO1DeCoDh0n7KGEdmMfWRh9eEtxY4"},
			args: `{"username": "bob", "password": "bob"}`,
		},
		{
			name: "with provided valid username and password, it returns OK, with replaced broken JWT, validation returns Unauthorized",
			exp: expected{statusCode: http.StatusOK, user: bobUser, err: nil,
				token: "brokenJWToken"},
			args: `{"username": "bob", "password": "bob"}`,
		},
		{
			name: "with provided invalid json payload, it returns BadRequest",
			exp:  expected{statusCode: http.StatusBadRequest, user: nil, err: nil},
			args: `username": "bob", "password": "bob"}`,
		},
		{
			name: "with no request body, it returns BadRequest",
			exp:  expected{statusCode: http.StatusBadRequest, user: nil, err: nil},
			args: `broken_pipe`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "http://127.0.0.1/lime/authenticate", bytes.NewBufferString(tt.args))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			if tt.args == `broken_pipe` {
				// simulates broken Read() on the Body
				request.Body = mockReadCloser{}
			}

			app := servicemocks.NewServiceProvider(s.T())
			app.On("GetUser", mock.AnythingOfType("string"), mock.AnythingOfType("string")).
				Return(tt.exp.user, tt.exp.err).Maybe()

			ep := NewEndPoint(s.ctx, s.vp, app)
			// call /lime/authenticate endpoint
			ep.Authenticate(response, request)

			require.Equal(t, tt.exp.statusCode, response.Code)

			if tt.exp.statusCode == http.StatusOK {
				require.NotEmpty(t, response.Body.String())

				// extract generated JWT for further inspection
				resp := new(responseAuthenticate)
				err := json.Unmarshal(response.Body.Bytes(), resp)
				require.NoError(t, err)

				// prepare new request to validate the token
				myRequest := httptest.NewRequest("GET", "http://127.0.0.1/lime/my", bytes.NewBufferString(""))
				myRequest.Header.Set("Content-Type", "application/json")
				myRequest.Header.Set(authTokenKey, resp.Token)

				// we also support tests for using old (expired) tokens
				if tt.exp.token != "" {
					myRequest.Header.Set(authTokenKey, tt.exp.token)
				}
				myResponse := httptest.NewRecorder()

				// build a testing endpoint with our Auth middleware and return the id of currently authenticated user
				authEndpoint := NewAuthBearerMiddleware(s.vp.GetString(cmd.JWTSecret), func(w http.ResponseWriter, r *http.Request) {
					userID, _ := r.Context().Value(userIDKey).(int)
					writeJSONResponse(w, http.StatusOK, userID)
				}, false)

				// call the testing endpoint
				authEndpoint.Authenticate(myResponse, myRequest)

				if tt.exp.token == "" {
					// check status code and compare the user id of the response
					require.Equal(t, http.StatusOK, myResponse.Code, resp.Token)
					require.Equal(t, strconv.Itoa(tt.exp.user.ID), myResponse.Body.String())
				} else {
					require.Equal(t, http.StatusUnauthorized, myResponse.Code, tt.exp.token)
				}
			} else if tt.exp.statusCode == http.StatusUnauthorized {
				require.Equal(t, "Unauthorized\n", response.Body.String())
			}
		})
	}
}

type mockReadCloser struct {
	io.ReadCloser
}

func (m mockReadCloser) Read(_ []byte) (n int, err error) {
	return 0, errors.New("broken pipe")
}

func mockSetupTransactions(txHashes []string) []*models.Transaction {
	txList := make([]*models.Transaction, 0, len(txHashes))
	for _, txHash := range txHashes {
		tx := &models.Transaction{
			TXHash:    txHash,
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
		}
		txList = append(txList, tx)
	}
	return txList
}

func TestEndpointTestSuite(t *testing.T) {
	suite.Run(t, new(EndpointTestSuite))
}
