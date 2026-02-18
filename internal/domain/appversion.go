package domain

type AppVersion struct {
	VersionID string `json:"id" dynamodbav:"version_id"`
	Version   string `json:"version" dynamodbav:"version"`
	Enable    bool   `json:"enable" dynamodbav:"enable"`
}
