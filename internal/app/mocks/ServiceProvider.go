// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	models "ethereum-fetcher/internal/store/pg/models"

	mock "github.com/stretchr/testify/mock"
)

// ServiceProvider is an autogenerated mock type for the ServiceProvider type
type ServiceProvider struct {
	mock.Mock
}

// GetAllTransactions provides a mock function with given fields:
func (_m *ServiceProvider) GetAllTransactions() ([]*models.Transaction, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetAllTransactions")
	}

	var r0 []*models.Transaction
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]*models.Transaction, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []*models.Transaction); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*models.Transaction)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMyTransactions provides a mock function with given fields: userID
func (_m *ServiceProvider) GetMyTransactions(userID int) ([]*models.Transaction, error) {
	ret := _m.Called(userID)

	if len(ret) == 0 {
		panic("no return value specified for GetMyTransactions")
	}

	var r0 []*models.Transaction
	var r1 error
	if rf, ok := ret.Get(0).(func(int) ([]*models.Transaction, error)); ok {
		return rf(userID)
	}
	if rf, ok := ret.Get(0).(func(int) []*models.Transaction); ok {
		r0 = rf(userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*models.Transaction)
		}
	}

	if rf, ok := ret.Get(1).(func(int) error); ok {
		r1 = rf(userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionsByHashes provides a mock function with given fields: txHashes, userID
func (_m *ServiceProvider) GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error) {
	ret := _m.Called(txHashes, userID)

	if len(ret) == 0 {
		panic("no return value specified for GetTransactionsByHashes")
	}

	var r0 []*models.Transaction
	var r1 error
	if rf, ok := ret.Get(0).(func([]string, int) ([]*models.Transaction, error)); ok {
		return rf(txHashes, userID)
	}
	if rf, ok := ret.Get(0).(func([]string, int) []*models.Transaction); ok {
		r0 = rf(txHashes, userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*models.Transaction)
		}
	}

	if rf, ok := ret.Get(1).(func([]string, int) error); ok {
		r1 = rf(txHashes, userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetUser provides a mock function with given fields: username, password
func (_m *ServiceProvider) GetUser(username string, password string) (*models.User, error) {
	ret := _m.Called(username, password)

	if len(ret) == 0 {
		panic("no return value specified for GetUser")
	}

	var r0 *models.User
	var r1 error
	if rf, ok := ret.Get(0).(func(string, string) (*models.User, error)); ok {
		return rf(username, password)
	}
	if rf, ok := ret.Get(0).(func(string, string) *models.User); ok {
		r0 = rf(username, password)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*models.User)
		}
	}

	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(username, password)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewServiceProvider creates a new instance of ServiceProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewServiceProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *ServiceProvider {
	mock := &ServiceProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
