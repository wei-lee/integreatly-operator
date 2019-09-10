package providers

import (
	"fmt"
	"github.com/pkg/errors"
)

type CloudServiceProvider interface {
	CreateCloudStorage(name string)error
	RemoveCloudStorage(name string)error
	ListCloudStorage()error
	CreateCloudDB(c CloudDB)(*CloudDBCoords, error)
	CreateCloudCache(c CloudCache )(*CloudCacheCoords, error)
	RemoveCloudCache(c CloudCache )error
	RemoveCloudDB(c CloudDB)error
}



// Not sure how viable this is as all the different providers may have different options.
type CloudDB struct {
	Type int
	StorageSize int64
	RetentionPeriod int64
	ClusterName, DBName string
	Engine int
	EngineVersion string
}

type CloudCache struct {
	Type int
	ClusterName string
	Engine int
	EngineVersion string
}

type CloudDBCoords struct {
 CloudCoords
}

type CloudCacheCoords struct {
	CloudCoords
}

type CloudCoords struct {
	Username, Password, Host string
	Port int64
}

func (cdc *CloudDBCoords)String()string  {
	return fmt.Sprintf("%s %s %s", cdc.Username, cdc.Password, cdc.Host)
}

const (
	CloudDBTypeDev  = iota
	CloudDBTypeProd
	CloudCacheTypeDev
)
const(
	CloudDbEnginePostgres = iota
	CloudCacheEngineRedis
)

type CloudProviderFactory struct {

}


const(
	ProviderNameAWS = "aws"
)


func (cpr CloudProviderFactory)Get(name string )(CloudServiceProvider, error)  {
	if name == ProviderNameAWS{
		return NewAWSServices()
	}
	return nil, errors.New("no such cloud provider "+ name)
}

