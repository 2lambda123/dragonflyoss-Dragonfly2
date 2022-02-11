/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package e2e

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint
	. "github.com/onsi/gomega"    //nolint

	"d7y.io/dragonfly/v2/client/clientutil"
	"d7y.io/dragonfly/v2/test/e2e/e2eutil"
)

var _ = Describe("Download with dfget and proxy", func() {
	Context("dfget", func() {
		singleDfgetTest("dfget daemon download should be ok",
			dragonflyNamespace, "component=dfdaemon",
			"dragonfly-dfdaemon-", "dfdaemon")
		for i := 0; i < 3; i++ {
			singleDfgetTest(
				fmt.Sprintf("dfget daemon proxy-%d should be ok", i),
				dragonflyE2ENamespace,
				fmt.Sprintf("statefulset.kubernetes.io/pod-name=proxy-%d", i),
				"proxy-", "proxy")
		}
	})
})

func getFileDetails() map[string]int {
	var details = map[string]int{}
	for _, path := range e2eutil.GetFileList() {
		out, err := e2eutil.DockerCommand("stat", "--printf=%s", path).CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		size, err := strconv.Atoi(string(out))
		Expect(err).NotTo(HaveOccurred())
		details[path] = size
	}
	return details
}

func getRandomRange(size int) *clientutil.Range {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	r1 := rnd.Intn(size - 1)
	r2 := rnd.Intn(size - 1)
	var start, end int
	if r1 > r2 {
		start, end = r2, r1
	} else {
		start, end = r1, r2
	}

	// range for [start, end]
	rg := &clientutil.Range{
		Start:  int64(start),
		Length: int64(end + 1 - start),
	}
	return rg
}

func singleDfgetTest(name, ns, label, podNamePrefix, container string) {
	It(name, func() {
		out, err := e2eutil.KubeCtlCommand("-n", ns, "get", "pod", "-l", label,
			"-o", "jsonpath='{range .items[*]}{.metadata.name}{end}'").CombinedOutput()
		podName := strings.Trim(string(out), "'")
		Expect(err).NotTo(HaveOccurred())
		fmt.Println("test in pod: " + podName)
		Expect(strings.HasPrefix(podName, podNamePrefix)).Should(BeTrue())
		pod := e2eutil.NewPodExec(ns, podName, container)
		// install curl
		_, err = pod.Command("apk", "add", "-U", "curl").CombinedOutput()
		Expect(err).NotTo(HaveOccurred())

		for path, size := range getFileDetails() {
			url1 := e2eutil.GetFileURL(path)
			url2 := e2eutil.GetNoContentLengthFileURL(path)

			// make ranged requests to invoke prefetch feature
			if featureGates.Enabled(featureGateRange) {
				rg := getRandomRange(size)
				downloadSingleFile(ns, pod, path, url1, size, rg)
				downloadSingleFile(ns, pod, path, url2, size, rg)
			}
			downloadSingleFile(ns, pod, path, url1, size, nil)
			downloadSingleFile(ns, pod, path, url2, size, nil)
		}
	})
}

func downloadSingleFile(ns string, pod *e2eutil.PodExec, path, url string, size int, rg *clientutil.Range) {
	var (
		sha256sum []string
		dfget     []string
		curl      []string
	)

	if rg == nil {
		sha256sum = append(sha256sum, "/usr/bin/sha256sum", path)
		dfget = append(dfget, "/opt/dragonfly/bin/dfget", "-O", "/tmp/d7y.out", url)
		curl = append(curl, "/usr/bin/curl", "-x", "http://127.0.0.1:65001", "-s", "--dump-header", "-", "-o", "/tmp/curl.out", url)
	} else {
		sha256sum = append(sha256sum, "sh", "-c",
			fmt.Sprintf("dd if=%s ibs=1 skip=%d count=%d 2> /dev/null | /usr/bin/sha256sum", path, rg.Start, rg.Length))
		dfget = append(dfget, "/opt/dragonfly/bin/dfget", "-O", "/tmp/d7y.out", "-H",
			fmt.Sprintf("Range: bytes=%d-%d", rg.Start, rg.Start+rg.Length-1), url)
		curl = append(curl, "/usr/bin/curl", "-x", "http://127.0.0.1:65001", "-s", "--dump-header", "-", "-o", "/tmp/curl.out",
			"--header", fmt.Sprintf("Range: bytes=%d-%d", rg.Start, rg.Start+rg.Length-1), url)
	}

	fmt.Printf("--------------------------------------------------------------------------------\n\n")
	if rg == nil {
		fmt.Printf("download size %d\n", size)
	} else {
		fmt.Printf("download range: bytes=%d-%d/%d, target length: %d\n",
			rg.Start, rg.Start+rg.Length-1, size, rg.Length)
	}
	// get original file digest
	out, err := e2eutil.DockerCommand(sha256sum...).CombinedOutput()
	fmt.Println("original sha256sum: " + string(out))
	Expect(err).NotTo(HaveOccurred())
	sha256sum1 := strings.Split(string(out), " ")[0]

	var (
		start time.Time
		end   time.Time
	)
	// download file via dfget
	start = time.Now()
	out, err = pod.Command(dfget...).CombinedOutput()
	end = time.Now()
	fmt.Println(string(out))
	Expect(err).NotTo(HaveOccurred())

	// get dfget downloaded file digest
	out, err = pod.Command("/usr/bin/sha256sum", "/tmp/d7y.out").CombinedOutput()
	fmt.Println("dfget sha256sum: " + string(out))
	Expect(err).NotTo(HaveOccurred())
	sha256sum2 := strings.Split(string(out), " ")[0]
	Expect(sha256sum1).To(Equal(sha256sum2))

	// slow download
	Expect(end.Sub(start).Seconds() < 30.0).To(Equal(true))

	// skip dfdaemon
	if ns == dragonflyNamespace {
		fmt.Println("skip " + dragonflyNamespace + " namespace proxy tests")
		return
	}
	// download file via proxy
	start = time.Now()
	out, err = pod.Command(curl...).CombinedOutput()
	end = time.Now()
	fmt.Print(string(out))
	Expect(err).NotTo(HaveOccurred())

	// get proxy downloaded file digest
	out, err = pod.Command("/usr/bin/sha256sum", "/tmp/curl.out").CombinedOutput()
	fmt.Println("curl sha256sum: " + string(out))
	Expect(err).NotTo(HaveOccurred())
	sha256sum3 := strings.Split(string(out), " ")[0]
	Expect(sha256sum1).To(Equal(sha256sum3))

	// slow download
	Expect(end.Sub(start).Seconds() < 30.0).To(Equal(true))
}
