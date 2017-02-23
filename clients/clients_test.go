package clients_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"time"

	"github.com/pivotal-cf/downtimer/clients"
	"github.com/pivotal-cf/downtimer/clients/clientsfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const sampleRecordFile = `timestamp,success,latency,code,size,fill,annotation
123,1,125.75904ms,200,79,
456,1,2.860896ms,200,79,
789,1,2.564204ms,200,79,
101112,1,3.562018ms,200,79,
131415,1,3.568ms,200,79,
`

var _ = Describe("Clients", func() {
	var prober *clients.Prober
	var opts clients.Opts
	var recordFile *os.File
	var err error
	var bosh *clientsfakes.FakeBosh
	BeforeEach(func() {
		recordFile, err = ioutil.TempFile("", "downtime-report.csv")
		Expect(err).NotTo(HaveOccurred())
		recordFile.Write([]byte(sampleRecordFile))
		opts = clients.Opts{
			OutputFile: recordFile.Name(),
			URL:        "http://localhost:54321/fake-url",
		}
		bosh = new(clientsfakes.FakeBosh)
	})
	JustBeforeEach(func() {
		prober, err = clients.NewProber(&opts, bosh)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.Remove(recordFile.Name())).ToNot(HaveOccurred())
	})

	Describe("Prober", func() {
		Context("recording downtime for given duration", func() {
			BeforeEach(func() {
				opts.Duration = 100*time.Millisecond + 2*time.Millisecond
				opts.Interval = 5 * time.Millisecond
			})
			It("records n = (duration/interval) times", func() {
				buf := make([]byte, 32*1024)
				prober.RecordDowntime()
				outputFile, err := os.Open(opts.OutputFile)
				Expect(err).NotTo(HaveOccurred())
				readBytesCount, err := outputFile.Read(buf)
				Expect(err).NotTo(HaveOccurred())
				lineCount := bytes.Count(buf[:readBytesCount], []byte{'\n'})
				Expect(lineCount).To(Equal(20 + 1)) // +1 for header
			})
		})
		Context("recording downtime for running deployment", func() {
			Context("when deployment isn't running anymore", func() {
				BeforeEach(func() {
					opts.Duration = 0 * time.Second
					opts.Interval = 5 * time.Millisecond
					opts.BoshTask = "111"

					bosh.GetCurrentTaskIdStub = func() (int, error) {
						return 0, nil
					}

				})
				It("should not record anything ", func() {
					buf := make([]byte, 32*1024)
					prober.RecordDowntime()
					outputFile, err := os.Open(opts.OutputFile)
					Expect(err).NotTo(HaveOccurred())
					_, err = outputFile.Read(buf)
					Expect(err).To(MatchError("EOF"))
				})
			})
			Context("when deployment is ongoing", func() {
				BeforeEach(func() {
					opts.Duration = 0 * time.Second
					opts.Interval = 5 * time.Millisecond
					opts.BoshTask = "111"

					validTaskCount := 4

					bosh.GetCurrentTaskIdStub = func() (int, error) {
						if validTaskCount > 0 {
							validTaskCount -= 1
							return 111, nil
						}
						return 0, nil
					}

				})
				It("should record for the duration of deployment", func() {
					buf := make([]byte, 32*1024)
					prober.RecordDowntime()
					outputFile, err := os.Open(opts.OutputFile)
					Expect(err).NotTo(HaveOccurred())
					readBytesCount, err := outputFile.Read(buf)
					Expect(err).NotTo(HaveOccurred())
					lineCount := bytes.Count(buf[:readBytesCount], []byte{'\n'})
					Expect(lineCount).To(Equal(4 + 1)) // +1 for header
				})
			})
		})
		Context("annotating the file", func() {
			var deploymentTimes clients.DeploymentTimes
			BeforeEach(func() {
				deploymentTimes = clients.DeploymentTimes{}
				deploymentTimes[123] = []string{"doppler done", "diego start"}
			})
			It("doesn't choke on a CSV header", func() {
				err := prober.AnnotateWithTimestamps(deploymentTimes)
				Expect(err).NotTo(HaveOccurred())
				f, err := os.Open(opts.OutputFile)
				Expect(err).NotTo(HaveOccurred())
				rewrittenFile, err := ioutil.ReadAll(f)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(rewrittenFile)).To(ContainSubstring("doppler done"))
			})
		})
	})
})
