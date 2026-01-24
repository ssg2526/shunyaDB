package config

import (
	"github.com/spf13/viper"
)

var ShunyaConfigs *ShunyaDBConfig

type ShunyaDBConfig struct {
	Host                     string `mapstructure:"host"`
	Port                     int    `mapstructure:"port"`
	WALDir                   string `mapstructure:"wal_dir"`
	WALShouldFsync           bool   `mapstructure:"wal_should_fsync"`
	WALBufSyncIntervalMillis int    `mapstructure:"wal_flush_interval_millis"`
	WALMaxSegmentSize        int    `mapstructure:"wal_max_segment_size"`
	WALWriteBufferSize       int    `mapstructure:"wal_write_buffer_size"`
	MemTableType             string `mapstructure:"mem_table_type"`
	StoreMetaDir             string `mapstructure:"store_meta_dir"`
	SSTableDir               string `mapstructure:"sst_dir"`
	SSTWriteBufferSize       int    `mapstructure:"sst_write_buffer_size"`
}

func InitConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	// checks the below paths for config.yaml file
	viper.AddConfigPath("../")
	viper.AddConfigPath("/etc/shunyaDB")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	if err := viper.Unmarshal(&ShunyaConfigs); err != nil {
		panic(err)
	}
}
