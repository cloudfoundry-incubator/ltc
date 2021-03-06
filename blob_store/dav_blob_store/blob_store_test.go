package dav_blob_store_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/ltc/blob_store/blob"
	"github.com/cloudfoundry-incubator/ltc/blob_store/dav_blob_store"
	config_package "github.com/cloudfoundry-incubator/ltc/config"
)

var _ = Describe("BlobStore", func() {
	var (
		blobStore      *dav_blob_store.BlobStore
		fakeServer     *ghttp.Server
		blobTargetInfo config_package.BlobStoreConfig
	)

	BeforeEach(func() {
		fakeServer = ghttp.NewServer()
		fakeServerURL, err := url.Parse(fakeServer.URL())
		Expect(err).NotTo(HaveOccurred())

		serverHost, serverPort, err := net.SplitHostPort(fakeServerURL.Host)
		Expect(err).NotTo(HaveOccurred())

		blobTargetInfo = config_package.BlobStoreConfig{
			Host:     serverHost,
			Port:     serverPort,
			Username: "user",
			Password: "pass",
		}

		blobStore = dav_blob_store.New(blobTargetInfo)
	})

	AfterEach(func() {
		if fakeServer != nil {
			fakeServer.Close()
		}
	})

	Describe("#List", func() {
		var responseBodyRoot string
		BeforeEach(func() {
			responseBodyRoot = `
				<?xml version="1.0" encoding="utf-8"?>
				<D:multistatus xmlns:D="DAV:" xmlns:ns0="urn:uuid:c2f41010-65b3-11d1-a29f-00aa00c14882/">
				  <D:response>
					<D:href>http://192.168.11.11:8444/blobs/a-droplet.tgz</D:href>
					<D:propstat>
					  <D:prop>
						<D:creationdate ns0:dt="dateTime.tz">2015-07-29T18:43:50Z</D:creationdate>
						<D:getcontentlanguage>en</D:getcontentlanguage>
						<D:getcontentlength>4096</D:getcontentlength>
						<D:getcontenttype>application/x-gtar-compressed</D:getcontenttype>
						<D:getlastmodified ns0:dt="dateTime.rfc1123">Wed, 29 Jul 2015 18:43:36 GMT</D:getlastmodified>
						<D:resourcetype>
						  <D:collection/>
						</D:resourcetype>
					  </D:prop>
					  <D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				  </D:response>
				  <D:response>
					<D:href>http://192.168.11.11:8444/blobs/b-droplet.tgz</D:href>
					<D:propstat>
					  <D:prop>
						<D:creationdate ns0:dt="dateTime.tz">2015-07-29T18:43:50Z</D:creationdate>
						<D:getcontentlanguage>en</D:getcontentlanguage>
						<D:getcontentlength>4096</D:getcontentlength>
						<D:getcontenttype>application/x-gtar-compressed</D:getcontenttype>
						<D:getlastmodified ns0:dt="dateTime.rfc1123">Wed, 29 Jul 2015 18:43:36 GMT</D:getlastmodified>
						<D:resourcetype>
						  <D:collection/>
						</D:resourcetype>
					  </D:prop>
					  <D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				  </D:response>
				  <D:response>
					<D:href>http://192.168.11.11:8444/blobs/c-droplet.tgz</D:href>
					<D:propstat>
					  <D:prop>
						<D:creationdate ns0:dt="dateTime.tz">2015-07-29T18:43:50Z</D:creationdate>
						<D:getcontentlanguage>en</D:getcontentlanguage>
						<D:getcontentlength>4096</D:getcontentlength>
						<D:getcontenttype>application/x-gtar-compressed</D:getcontenttype>
						<D:getlastmodified ns0:dt="dateTime.rfc1123">Wed, 29 Jul 2015 18:43:36 GMT</D:getlastmodified>
						<D:resourcetype>
						  <D:collection/>
						</D:resourcetype>
					  </D:prop>
					  <D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				  </D:response>
				</D:multistatus>
			`

			responseBodyRoot = strings.Replace(responseBodyRoot, "http://192.168.11.11:8444", fakeServer.URL(), -1)
		})

		It("lists objects", func() {
			fakeServer.RouteToHandler("PROPFIND", "/blobs", ghttp.CombineHandlers(
				ghttp.VerifyHeaderKV("Depth", "1"),
				ghttp.VerifyBasicAuth("user", "pass"),
				ghttp.RespondWith(207, responseBodyRoot, http.Header{"Content-Type": []string{"application/xml"}}),
			))

			expectedTime, err := time.Parse(time.RFC1123, "Wed, 29 Jul 2015 18:43:36 GMT")
			Expect(err).NotTo(HaveOccurred())

			Expect(blobStore.List()).To(ConsistOf(
				blob.Blob{Path: "b-droplet.tgz", Size: 4096, Created: expectedTime},
				blob.Blob{Path: "a-droplet.tgz", Size: 4096, Created: expectedTime},
				blob.Blob{Path: "c-droplet.tgz", Size: 4096, Created: expectedTime},
			))

			Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
		})

		Context("when the list call fails", func() {
			It("returns an error it can't connect to the server", func() {
				fakeServer.Close()
				fakeServer = nil

				_, err := blobStore.List()
				Expect(reflect.TypeOf(err).String()).To(Equal("*net.OpError"))
			})

			It("returns an error when we fail to retrieve the objects from DAV", func() {
				fakeServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("PROPFIND", "/blobs"),
					ghttp.VerifyHeaderKV("Depth", "1"),
					ghttp.VerifyBasicAuth("user", "pass"),
					ghttp.RespondWith(http.StatusInternalServerError, nil, http.Header{"Content-Type": []string{"application/xml"}}),
				))

				_, err := blobStore.List()
				Expect(err).To(MatchError(ContainSubstring("Internal Server Error")))

				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("returns an error when it fails to parse the XML", func() {
				fakeServer.RouteToHandler("PROPFIND", "/blobs", ghttp.CombineHandlers(
					ghttp.VerifyHeaderKV("Depth", "1"),
					ghttp.VerifyBasicAuth("user", "pass"),
					ghttp.RespondWith(207, `<D:bad`, http.Header{"Content-Type": []string{"application/xml"}}),
				))

				_, err := blobStore.List()
				Expect(err).To(MatchError("XML syntax error on line 1: unexpected EOF"))

				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			Context("when it fails to parse the time", func() {
				It("returns an error", func() {
					responseBodyRoot = strings.Replace(responseBodyRoot, "Wed, 29 Jul 2015 18:43:36 GMT", "ABC", -1)

					fakeServer.RouteToHandler("PROPFIND", "/blobs", ghttp.CombineHandlers(
						ghttp.VerifyHeaderKV("Depth", "1"),
						ghttp.VerifyBasicAuth("user", "pass"),
						ghttp.RespondWith(207, responseBodyRoot, http.Header{"Content-Type": []string{"application/xml"}}),
					))

					_, err := blobStore.List()
					Expect(err).To(MatchError(ContainSubstring(`cannot parse "ABC"`)))

					Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})

		It("uses the correct HTTP client with a specified timeout", func() {
			defaultTransport := blobStore.Client.Transport
			defer func() { blobStore.Client.Transport = defaultTransport }()

			usedClient := false
			blobStore.Client.Transport = &http.Transport{
				Proxy: func(request *http.Request) (*url.URL, error) {
					usedClient = true
					return nil, errors.New("some error")
				},
			}

			blobStore.List()
			Expect(usedClient).To(BeTrue())
		})
	})

	Describe("#Upload", func() {
		It("uploads the provided reader into the collection", func() {
			fakeServer.RouteToHandler("PUT", "/blobs/some-object", ghttp.CombineHandlers(
				ghttp.VerifyBasicAuth("user", "pass"),
				func(_ http.ResponseWriter, request *http.Request) {
					Expect(ioutil.ReadAll(request.Body)).To(Equal([]byte("some data")))
				},
				ghttp.RespondWith(http.StatusCreated, "", http.Header{}),
			))

			Expect(blobStore.Upload("some-object", strings.NewReader("some data"))).To(Succeed())

			Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
		})

		It("returns an error when DAV fails to receive the object", func() {
			fakeServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/blobs/some-object"),
				ghttp.VerifyBasicAuth("user", "pass"),
				ghttp.RespondWith(http.StatusInternalServerError, "", http.Header{}),
			))

			err := blobStore.Upload("some-object", strings.NewReader("some data"))
			Expect(err).To(MatchError(ContainSubstring("500 Internal Server Error")))

			Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
		})

		It("returns an error when the DAV client cannot connect", func() {
			fakeServer.Close()
			fakeServer = nil

			err := blobStore.Upload("some-object", strings.NewReader("some data"))
			Expect(reflect.TypeOf(err).String()).To(Equal("*url.Error"))
		})
	})

	Describe("#Download", func() {
		It("dowloads the requested path", func() {
			fakeServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/blobs/some-object"),
				ghttp.VerifyBasicAuth("user", "pass"),
				ghttp.RespondWith(http.StatusOK, "some data", http.Header{"Content-Length": []string{"9"}}),
			))

			pathReader, err := blobStore.Download("some-object")
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(pathReader)).To(Equal([]byte("some data")))
			Expect(pathReader.Close()).To(Succeed())

			Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
		})

		It("returns an error when DAV fails to retrieve the object", func() {
			fakeServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/blobs/some-object"),
				ghttp.VerifyBasicAuth("user", "pass"),
				ghttp.RespondWith(http.StatusInternalServerError, "", http.Header{}),
			))

			_, err := blobStore.Download("some-object")
			Expect(err).To(MatchError(ContainSubstring("500 Internal Server Error")))
		})

		It("returns an error when the DAV client cannot connect", func() {
			fakeServer.Close()
			fakeServer = nil

			_, err := blobStore.Download("some-object")
			Expect(reflect.TypeOf(err).String()).To(Equal("*url.Error"))
		})
	})

	Describe("#Delete", func() {
		It("deletes the object at the provided path", func() {
			fakeServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("DELETE", "/blobs/some-path/some-object"),
				ghttp.VerifyBasicAuth("user", "pass"),
				ghttp.RespondWith(http.StatusNoContent, ""),
			))
			Expect(blobStore.Delete("some-path/some-object")).NotTo(HaveOccurred())
			Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
		})

		It("returns an error when DAV fails to delete the object", func() {
			fakeServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("DELETE", "/blobs/some-path/some-object"),
				ghttp.VerifyBasicAuth("user", "pass"),
				ghttp.RespondWith(http.StatusInternalServerError, "", http.Header{}),
			))

			err := blobStore.Delete("some-path/some-object")
			Expect(err).To(MatchError(ContainSubstring("500 Internal Server Error")))
		})

		It("returns an error when the DAV client cannot connect", func() {
			fakeServer.Close()
			fakeServer = nil

			err := blobStore.Delete("some-path/some-object")
			Expect(reflect.TypeOf(err).String()).To(Equal("*net.OpError"))
		})
	})

	Context("Droplet Actions", func() {
		var dropletURL string

		BeforeEach(func() {
			dropletURL = fmt.Sprintf("http://%s:%s@%s:%s/blobs/droplet-name", blobTargetInfo.Username, blobTargetInfo.Password, blobTargetInfo.Host, blobTargetInfo.Port)
		})

		Describe("#DownloadAppBitsAction", func() {
			It("constructs the correct Action to download app bits", func() {
				Expect(blobStore.DownloadAppBitsAction("droplet-name")).To(Equal(models.WrapAction(&models.DownloadAction{
					From:      dropletURL + "-bits.zip",
					To:        "/tmp/app",
					User:      "vcap",
					LogSource: "DROPLET",
				})))
			})
		})

		Describe("#DeleteAppBitsAction", func() {
			It("constructs the correct Action to delete app bits", func() {
				Expect(blobStore.DeleteAppBitsAction("droplet-name")).To(Equal(models.WrapAction(&models.RunAction{
					Path:      "/tmp/davtool",
					Dir:       "/",
					Args:      []string{"delete", dropletURL + "-bits.zip"},
					User:      "vcap",
					LogSource: "DROPLET",
				})))
			})
		})

		Describe("#UploadDropletAction", func() {
			It("constructs the correct Action to upload the droplet", func() {
				Expect(blobStore.UploadDropletAction("droplet-name")).To(Equal(models.WrapAction(&models.RunAction{
					Path:      "/tmp/davtool",
					Dir:       "/",
					Args:      []string{"put", dropletURL + "-droplet.tgz", "/tmp/droplet"},
					User:      "vcap",
					LogSource: "DROPLET",
				})))
			})
		})

		Describe("#DownloadDropletAction", func() {
			It("constructs the correct Action to download the droplet", func() {
				Expect(blobStore.DownloadDropletAction("droplet-name")).To(Equal(models.WrapAction(&models.DownloadAction{
					From:      dropletURL + "-droplet.tgz",
					To:        "/home/vcap",
					User:      "vcap",
					LogSource: "DROPLET",
				})))
			})
		})
	})
})
