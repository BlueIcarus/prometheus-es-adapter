package elasticsearch

const sampleType = "_doc"

const indexCreate = `{
	"aliases": {
		"{{.Alias}}": {}
	}
}`

const indexTemplate = `{
"template": {
  "settings": {
    "number_of_shards": {{.Shards}},
    "number_of_replicas": {{.Replicas}}
  },
  "mappings": {
    "_source": {
      "enabled": true,
      "includes": [],
      "excludes": []
    },
    "_routing": {
      "required": false
    },
    "dynamic": true,
    "numeric_detection": false,
    "date_detection": true,
    "dynamic_date_formats": [
      "strict_date_optional_time",
      "yyyy/MM/dd HH:mm:ss Z||yyyy/MM/dd Z"
    ],
    "dynamic_templates": [
      {
        "mappings": {
          "match_mapping_type": "string",
          "path_match": "label.*",
          "mapping": {
            "type": "keyword"
          }
        }
      }
    ],
    "properties": {
      "timestamp": {
        "type": "date",
        "format": "strict_date_optional_time||epoch_millis"
      },
	  "value": {
        "type": "binary",
      }
    }
  }
},
"index_patterns": [
  "{{.Alias}}-*"
]
}`
