// /*
// Copyright © 2022 - 2023 SUSE LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
//

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host (interfaces: Manager)
//
// Generated by this command:
//
//	mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/host/host_mocks.go -package=host github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host Manager
//
// Package host is a generated GoMock package.
package host

import (
	reflect "reflect"

	v1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	gomock "go.uber.org/mock/gomock"
)

// MockManager is a mock of Manager interface.
type MockManager struct {
	ctrl     *gomock.Controller
	recorder *MockManagerMockRecorder
}

// MockManagerMockRecorder is the mock recorder for MockManager.
type MockManagerMockRecorder struct {
	mock *MockManager
}

// NewMockManager creates a new mock instance.
func NewMockManager(ctrl *gomock.Controller) *MockManager {
	mock := &MockManager{ctrl: ctrl}
	mock.recorder = &MockManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManager) EXPECT() *MockManagerMockRecorder {
	return m.recorder
}

// GetCurrentHostname mocks base method.
func (m *MockManager) GetCurrentHostname() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCurrentHostname")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCurrentHostname indicates an expected call of GetCurrentHostname.
func (mr *MockManagerMockRecorder) GetCurrentHostname() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCurrentHostname", reflect.TypeOf((*MockManager)(nil).GetCurrentHostname))
}

// PickHostname mocks base method.
func (m *MockManager) PickHostname(arg0 v1beta1.Hostname) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PickHostname", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PickHostname indicates an expected call of PickHostname.
func (mr *MockManagerMockRecorder) PickHostname(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PickHostname", reflect.TypeOf((*MockManager)(nil).PickHostname), arg0)
}

// PowerOff mocks base method.
func (m *MockManager) PowerOff() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PowerOff")
	ret0, _ := ret[0].(error)
	return ret0
}

// PowerOff indicates an expected call of PowerOff.
func (mr *MockManagerMockRecorder) PowerOff() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PowerOff", reflect.TypeOf((*MockManager)(nil).PowerOff))
}

// Reboot mocks base method.
func (m *MockManager) Reboot() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Reboot")
	ret0, _ := ret[0].(error)
	return ret0
}

// Reboot indicates an expected call of Reboot.
func (mr *MockManagerMockRecorder) Reboot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reboot", reflect.TypeOf((*MockManager)(nil).Reboot))
}

// SetHostname mocks base method.
func (m *MockManager) SetHostname(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetHostname", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHostname indicates an expected call of SetHostname.
func (mr *MockManagerMockRecorder) SetHostname(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHostname", reflect.TypeOf((*MockManager)(nil).SetHostname), arg0)
}
