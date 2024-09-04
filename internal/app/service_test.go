package app

import (
	"context"
	"fmt"
	"testing"

	"ethereum-fetcher/cmd"
	netmocks "ethereum-fetcher/internal/network/mocks"
	storagemocks "ethereum-fetcher/internal/store/mocks"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/ericlagergren/decimal"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/types"
)

// This test suite is added to satisfy the following requirement:
//
// "Create a unit test suite that proves that:
//   - Your service level code works"
//
// In order to do that, we are using mocks for both storage (database) and network (ethereum node)
type ServiceTestSuite struct {
	suite.Suite
	vp  *viper.Viper
	ctx context.Context
}

// this function executes before the test suite begins execution
func (s *ServiceTestSuite) SetupSuite() {
	vp := cmd.NewViper()
	s.vp = vp

	cmd.LogInit("fatal")
}

// this function executes before each test case
func (s *ServiceTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ServiceTestSuite) TestGetUser() {
	t := s.T()

	user0 := mockUser(0)
	user1 := mockUser(2)

	type args struct {
		user *models.User
		err  error
	}
	tests := []struct {
		name     string
		args     args
		mockData args
		want     *models.User
		wantErr  bool
	}{
		{
			name: "with provided valid username and password, it returns successfully the txDB object",
			args: args{
				user: user1,
			},
			mockData: args{
				user: user1,
			},
			want:    user1,
			wantErr: false,
		},
		{
			name: "with provided valid username and password, it fails error due to db error",
			args: args{
				user: user1,
			},
			mockData: args{
				user: nil,
				err:  fmt.Errorf("failed to execute a one query for users"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "with provided incorrect username or password, it returns non-authenticated txDB",
			args: args{
				user: user1,
			},
			mockData: args{
				user: user0,
				err:  nil,
			},
			want:    user0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := storagemocks.NewStorageProvider(s.T())
			net := netmocks.NewEthereumProvider(s.T())

			st.On("GetUser", mock.AnythingOfType("string"), mock.AnythingOfType("string")).
				Return(tt.mockData.user, tt.mockData.err)
			appService := NewService(s.ctx, s.vp, st, net)

			freshUser, err := appService.GetUser(tt.args.user.Username, tt.args.user.Password)
			if !tt.wantErr {
				assert.Nil(t, err)

				assert.NotNil(t, freshUser)
				assert.Equal(t, tt.want, freshUser)
			} else {
				assert.Error(t, err)

				assert.Nil(t, freshUser)
			}
		})
	}
}

func (s *ServiceTestSuite) TestGetTransactionsByHashes() {
	t := s.T()

	txList := mockEthereumTransactions()

	type args struct {
		txHashes      []string
		userID        int
		txDB, netDB   []*models.Transaction
		errDB, errNet error
	}
	tests := []struct {
		name     string
		args     args
		mockData args
		want     []*models.Transaction
		wantErr  bool
	}{
		{
			name: "with provided list of tx hashes, it returns successfully the list of transactions, read from db, by user",
			args: args{
				txHashes: []string{txList[0].TXHash, txList[1].TXHash},
				userID:   2,
			},
			mockData: args{
				txDB: txList,
			},
			want:    txList,
			wantErr: false,
		},
		{
			name: "with provided list of tx hashes, it returns successfully the list of transactions, read from db, by non-authenticated user",
			args: args{
				txHashes: []string{txList[0].TXHash, txList[1].TXHash},
				userID:   0,
			},
			mockData: args{
				txDB: txList,
			},
			want:    txList,
			wantErr: false,
		},
		{
			name: "with provided list of tx hashes, it returns successfully the list of transactions, read from net, by user",
			args: args{
				txHashes: []string{txList[0].TXHash, txList[1].TXHash},
				userID:   2,
			},
			mockData: args{
				txDB:  []*models.Transaction{},
				netDB: txList,
				errDB: nil,
			},
			want:    txList,
			wantErr: false,
		},
		{
			name: "with provided list of tx hashes, it fails to fetch txs from net, by user",
			args: args{
				txHashes: []string{txList[0].TXHash, txList[1].TXHash},
				userID:   2,
			},
			mockData: args{
				txDB:   []*models.Transaction{},
				netDB:  nil,
				errDB:  nil,
				errNet: fmt.Errorf("error fetching ethereum tx data"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "with provided list of tx hashes, it fails to fetch txs from net, by non-authenticated user",
			args: args{
				txHashes: []string{txList[0].TXHash, txList[1].TXHash},
				userID:   0,
			},
			mockData: args{
				txDB:   []*models.Transaction{},
				netDB:  nil,
				errDB:  nil,
				errNet: fmt.Errorf("error fetching ethereum tx data"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "with provided list of tx hashes, it fails to fetch txs from db and returns db error",
			args: args{
				txHashes: []string{txList[0].TXHash, txList[1].TXHash},
			},
			mockData: args{
				txDB:  nil,
				errDB: fmt.Errorf("failed to assign all query results to Transaction slice"),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := storagemocks.NewStorageProvider(s.T())
			net := netmocks.NewEthereumProvider(s.T())

			st.On("GetTransactionsByHashes", mock.AnythingOfType("[]string"), mock.AnythingOfType("int")).
				Return(tt.mockData.txDB, tt.mockData.errDB)
			st.On("InsertTransactionsUser", mock.AnythingOfType("[]*models.Transaction"), mock.AnythingOfType("int")).
				Return(tt.mockData.errDB).Maybe()

			if tt.mockData.netDB != nil {
				net.On("GetTransactionByHash", mock.AnythingOfType("string")).Once().Return(tt.mockData.netDB[0], tt.mockData.errDB)
				net.On("GetTransactionByHash", mock.AnythingOfType("string")).Once().Return(tt.mockData.netDB[1], tt.mockData.errDB)

				if !tt.wantErr {
					st.On("InsertTransactions", mock.Anything, mock.Anything).
						Return(tt.mockData.errDB)
				}
			} else if tt.mockData.errNet != nil {
				net.On("GetTransactionByHash", mock.AnythingOfType("string")).Once().Return(nil, tt.mockData.errNet)
			}

			appService := NewService(s.ctx, s.vp, st, net)

			freshTxs, err := appService.GetTransactionsByHashes(tt.args.txHashes, tt.args.userID)
			if !tt.wantErr {
				assert.Nil(t, err)

				assert.NotNil(t, freshTxs)
				assert.Equal(t, tt.want, freshTxs)
			} else {
				assert.Error(t, err)

				assert.Nil(t, freshTxs)
			}
		})
	}
}

func (s *ServiceTestSuite) TestGetAllTransactions() {
	t := s.T()

	txList := mockEthereumTransactions()

	type args struct {
		tx  []*models.Transaction
		err error
	}
	tests := []struct {
		name     string
		args     args
		mockData args
		want     []*models.Transaction
		wantErr  bool
	}{
		{
			name: "with no arguments, it returns successfully the list of transactions",
			args: args{},
			mockData: args{
				tx: txList,
			},
			want:    txList,
			wantErr: false,
		},
		{
			name: "with no arguments, it returns db error",
			args: args{},
			mockData: args{
				tx:  nil,
				err: fmt.Errorf("failed to assign all query results to Transaction slice"),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := storagemocks.NewStorageProvider(s.T())
			net := netmocks.NewEthereumProvider(s.T())

			st.On("GetAllTransactions").
				Return(tt.mockData.tx, tt.mockData.err)
			appService := NewService(s.ctx, s.vp, st, net)

			freshTxs, err := appService.GetAllTransactions()
			if !tt.wantErr {
				assert.Nil(t, err)

				assert.NotNil(t, freshTxs)
				assert.Equal(t, tt.want, freshTxs)
			} else {
				assert.Error(t, err)

				assert.Nil(t, freshTxs)
			}
		})
	}
}

func (s *ServiceTestSuite) TestGetMyTransactions() {
	t := s.T()

	user1 := mockUser(1)
	txList := mockEthereumTransactions()

	type args struct {
		tx  []*models.Transaction
		err error
	}
	tests := []*struct {
		name     string
		args     args
		mockData args
		want     []*models.Transaction
		wantErr  bool
	}{
		{
			name: "with no arguments, it returns successfully the list of my transactions",
			args: args{},
			mockData: args{
				tx: txList,
			},
			want:    txList,
			wantErr: false,
		},
		{
			name: "with no arguments, it returns db error",
			args: args{},
			mockData: args{
				tx:  nil,
				err: fmt.Errorf("failed to assign all query results to Transaction slice"),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := storagemocks.NewStorageProvider(s.T())
			net := netmocks.NewEthereumProvider(s.T())

			st.On("GetMyTransactions", mock.AnythingOfType("int")).
				Return(tt.mockData.tx, tt.mockData.err)
			appService := NewService(s.ctx, s.vp, st, net)

			freshTxs, err := appService.GetMyTransactions(user1.ID)
			if !tt.wantErr {
				assert.Nil(t, err)

				assert.NotNil(t, freshTxs)
				assert.Equal(t, tt.want, freshTxs)
			} else {
				assert.Error(t, err)

				assert.Nil(t, freshTxs)
			}
		})
	}
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
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
