//go:generate mockgen -source=../../controllers/management/oidcprovider/controller.go -destination=./strgenerator.go -package=mocks
//go:generate mockgen -source=../provider/authorize.go -destination=./authorize.go -package=mocks
//go:generate mockgen -source=../provider/token.go -destination=./token.go -package=mocks

package mocks
