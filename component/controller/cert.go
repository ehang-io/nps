package controller

import (
	"crypto/x509/pkix"
	"ehang.io/nps/lib/cert"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"net/http"
)

type certServe struct {
	baseController
	cg *cert.X509Generator
}

func (cs *certServe) Init(rootCert []byte, rootKey []byte) error {
	cs.cg = cert.NewX509Generator(pkix.Name{
		Country:            []string{"cn"},
		Organization:       []string{"ehang"},
		OrganizationalUnit: []string{"nps"},
		Province:           []string{"beijing"},
		CommonName:         "nps",
		Locality:           []string{"beijing"},
	})
	return cs.cg.InitRootCa(rootCert, rootKey)
}

type certInfo struct {
	Name     string `json:"name"`
	Uuid     string `json:"uuid"`
	CertType string `json:"cert_type"`
	Cert     string `json:"cert"`
	Key      string `json:"key"`
	Sn       string `json:"sn"`
	Remark   string `json:"remark"`
	Status   int    `json:"status"`
}

// Create
// Cert type root|bridge|server|client|secret
func (cs *certServe) Create(c *gin.Context) {
	var ci certInfo
	err := c.BindJSON(&ci)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	crt, key, err := cs.cg.CreateCert(fmt.Sprintf("%s.nps.ehang.io", ci.CertType))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	sn, err := cert.GetCertSnFromEncode(crt)
	ci.Cert, ci.Key, ci.Sn, ci.Uuid = string(crt), string(key), sn, uuid.NewV4().String()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	b, err := json.Marshal(ci)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	err = cs.db.Insert("cert", ci.Uuid, string(b))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (cs *certServe) Update(c *gin.Context) {
	var ci certInfo
	err := c.BindJSON(&ci)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	b, err := json.Marshal(ci)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	err = cs.db.Update(cs.tableName, ci.Uuid, string(b))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}
