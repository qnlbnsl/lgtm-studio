// Command streamprobe plays both ends of a long two-way stream, so the stream test can prove
// that the web server rules pass such a stream on without cutting it.
//
// The mode serve stands in for the server, and the modes dial and shaped stand in for a machine.
// Both ends speak cleartext HTTP/2 (h2c) and exchange lines of text. The program uses the
// standard library alone.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// The names of the wire protocol between the two ends.
const (
	shapeHeader   = "Probe-Shape"
	ticksHeader   = "Probe-Ticks"
	trailerHeader = "Probe-End"
	markLine      = "mark"
	upClosed      = "up-closed"
	downClosed    = "down-closed"
)

// The time limits of the client side.
const (
	echoWait     = 10 * time.Second
	responseWait = 10 * time.Second
	tickWait     = 10 * time.Second
	answerWait   = 5 * time.Second
	endWait      = 10 * time.Second
	tickEvery    = 2 * time.Second
	defaultTicks = 5
)

func main() {
	mode := flag.String("mode", "", "serve, dial, or shaped")
	addr := flag.String("addr", ":8080", "address to listen on")
	url := flag.String("url", "", "address to dial")
	shape := flag.String("shape", "", "quiet-up or quiet-down")
	every := flag.Duration("every", 2*time.Second, "time between messages")
	total := flag.Duration("total", 20*time.Second, "active duration")
	gap := flag.Duration("gap", 0, "silent duration")
	flag.Parse()

	switch *mode {
	case "serve":
		serve(*addr)
	case "dial":
		dial(*url, *every, *total, *gap)
	case "shaped":
		shaped(*url, *shape, *total)
	default:
		fmt.Fprintln(os.Stderr, "Use -mode serve, dial, or shaped.")
		os.Exit(2)
	}
}

// serve listens with HTTP/1.1 and cleartext HTTP/2, and answers each request in the shape that
// its header Probe-Shape selects.
func serve(addr string) {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server := &http.Server{
		Addr:      addr,
		Handler:   http.HandlerFunc(handle),
		Protocols: &protocols,
	}
	fmt.Printf("upstream listening addr=%s\n", addr)
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "upstream stopped: %v\n", err)
		os.Exit(1)
	}
}

func handle(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("upstream proto=%s path=%s\n", r.Proto, r.URL.Path)
	// A body that may be read while the response is written is the point of this program.
	// HTTP/2 always allows it, and HTTP/1.1 needs it to be turned on.
	control := http.NewResponseController(w)
	_ = control.EnableFullDuplex()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Trailer", trailerHeader)

	shape := r.Header.Get(shapeHeader)
	var result string
	switch shape {
	case "":
		result = serveEcho(w, control, r.Body)
	case "quiet-down":
		result = serveQuietDown(w, control, r.Body)
	case "quiet-up":
		result = serveQuietUp(w, control, r.Body)
	case "ticks":
		result = serveTicks(w, control, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
		result = "unknown shape " + shape
	}
	fmt.Printf("upstream done path=%s shape=%q result=%s\n", r.URL.Path, shape, result)
}

// writeLine writes one line and flushes it, so it leaves at once.
func writeLine(w io.Writer, control *http.ResponseController, line string) error {
	if _, err := io.WriteString(w, line+"\n"); err != nil {
		return err
	}
	return control.Flush()
}

// serveEcho echoes each line of the body and sets the trailer to the number of lines.
func serveEcho(w http.ResponseWriter, control *http.ResponseController, body io.Reader) string {
	w.WriteHeader(http.StatusOK)
	if err := control.Flush(); err != nil {
		return "flush: " + err.Error()
	}
	lines := 0
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		lines++
		if err := writeLine(w, control, "echo "+scanner.Text()); err != nil {
			return "write: " + err.Error()
		}
	}
	if err := scanner.Err(); err != nil {
		return "read: " + err.Error()
	}
	w.Header().Set(trailerHeader, strconv.Itoa(lines))
	return "closed after " + strconv.Itoa(lines) + " lines"
}

// serveQuietDown writes nothing, not even the status, until the line mark arrives. It then
// writes the number of lines before mark while the body is still open, and sets the trailer
// down-closed when the body ends.
func serveQuietDown(w http.ResponseWriter, control *http.ResponseController, body io.Reader) string {
	lines := 0
	marked := false
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if marked {
			continue
		}
		if scanner.Text() != markLine {
			lines++
			continue
		}
		marked = true
		w.WriteHeader(http.StatusOK)
		if err := writeLine(w, control, "count "+strconv.Itoa(lines)); err != nil {
			return "write: " + err.Error()
		}
	}
	if err := scanner.Err(); err != nil {
		return "read: " + err.Error()
	}
	if !marked {
		w.WriteHeader(http.StatusOK)
	}
	w.Header().Set(trailerHeader, downClosed)
	return "closed after " + strconv.Itoa(lines) + " lines"
}

// serveQuietUp writes a tick every two seconds, acknowledges each line after the first, and
// sets the trailer up-closed when the body ends.
func serveQuietUp(w http.ResponseWriter, control *http.ResponseController, body io.Reader) string {
	w.WriteHeader(http.StatusOK)
	if err := control.Flush(); err != nil {
		return "flush: " + err.Error()
	}
	lines := make(chan string)
	ended := make(chan error, 1)
	// The signal done closes on every return, so the scanner never stays blocked on a send
	// after a failed write has ended the handler.
	done := make(chan struct{})
	defer close(done)
	go func() {
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-done:
				return
			}
		}
		ended <- scanner.Err()
	}()
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	ticks, received := 0, 0
	for {
		select {
		case <-ticker.C:
			ticks++
			if err := writeLine(w, control, "tick "+strconv.Itoa(ticks)); err != nil {
				return "write: " + err.Error()
			}
		case line := <-lines:
			received++
			if received > 1 {
				if err := writeLine(w, control, "ack "+line); err != nil {
					return "write: " + err.Error()
				}
			}
		case err := <-ended:
			if err != nil {
				return "read: " + err.Error()
			}
			w.Header().Set(trailerHeader, upClosed)
			return "closed after " + strconv.Itoa(ticks) + " ticks"
		}
	}
}

// serveTicks reads the whole body first, then writes as many ticks as the header Probe-Ticks
// asks for, one every two seconds.
func serveTicks(w http.ResponseWriter, control *http.ResponseController, r *http.Request) string {
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		return "read: " + err.Error()
	}
	count := defaultTicks
	if value := r.Header.Get(ticksHeader); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return "bad tick count " + strconv.Quote(value)
		}
		count = parsed
	}
	w.WriteHeader(http.StatusOK)
	if err := control.Flush(); err != nil {
		return "flush: " + err.Error()
	}
	for tick := 1; tick <= count; tick++ {
		select {
		case <-r.Context().Done():
			return "canceled after " + strconv.Itoa(tick-1) + " ticks"
		case <-time.After(tickEvery):
		}
		if err := writeLine(w, control, "tick "+strconv.Itoa(tick)); err != nil {
			return "write: " + err.Error()
		}
	}
	return "sent " + strconv.Itoa(count) + " ticks"
}

// event is one line of a response body, or the end of the body with its error.
type event struct {
	line string
	err  error
}

// stream is the client side of one two-way stream.
type stream struct {
	up       *io.PipeWriter
	response chan responseResult
	resp     *http.Response
	events   chan event
}

type responseResult struct {
	resp *http.Response
	err  error
}

// open starts a request whose body is a pipe, so lines can be sent while the response is read.
func open(url, shape string) *stream {
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	client := &http.Client{Transport: &http.Transport{Protocols: &protocols}}

	body, up := io.Pipe()
	request, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		fail("cannot make the request: %v", err)
	}
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if shape != "" {
		request.Header.Set(shapeHeader, shape)
	}
	s := &stream{up: up, response: make(chan responseResult, 1)}
	go func() {
		resp, err := client.Do(request)
		s.response <- responseResult{resp, err}
	}()
	return s
}

// await waits for the response status and starts reading the body line by line.
func (s *stream) await(wait time.Duration) {
	select {
	case result := <-s.response:
		s.accept(result)
	case <-time.After(wait):
		fail("no response within %s", wait)
	}
}

func (s *stream) accept(result responseResult) {
	if result.err != nil {
		fail("the request failed: %v", result.err)
	}
	if result.resp.StatusCode != http.StatusOK {
		fail("the response status is %d", result.resp.StatusCode)
	}
	s.resp = result.resp
	s.events = make(chan event, 64)
	go func() {
		reader := bufio.NewReader(s.resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err == nil {
				s.events <- event{line: strings.TrimSuffix(line, "\n")}
				continue
			}
			if line != "" && err == io.EOF {
				err = fmt.Errorf("the body ended inside the line %q", line)
			}
			s.events <- event{err: err}
			close(s.events)
			return
		}
	}()
}

// send writes one line up.
func (s *stream) send(line string) {
	if _, err := io.WriteString(s.up, line+"\n"); err != nil {
		fail("cannot send %s: %v", line, err)
	}
}

// next returns the next line, or fails when the body ends or nothing arrives in time.
func (s *stream) next(wait time.Duration, what string) string {
	select {
	case e := <-s.events:
		if e.err == io.EOF {
			fail("the stream ended while waiting for %s", what)
		}
		if e.err != nil {
			fail("the stream broke while waiting for %s: %v", what, e.err)
		}
		return e.line
	case <-time.After(wait):
		fail("%s did not arrive within %s", what, wait)
	}
	return ""
}

// finish closes the upward side, requires a clean end with no further line except the ones
// that allow accepts, and returns the trailer.
func (s *stream) finish(allow func(string) bool) string {
	if err := s.up.Close(); err != nil {
		fail("cannot close the upward side: %v", err)
	}
	deadline := time.After(endWait)
	for {
		select {
		case e := <-s.events:
			if e.err == io.EOF {
				return s.resp.Trailer.Get(trailerHeader)
			}
			if e.err != nil {
				fail("the stream broke at its end: %v", e.err)
			}
			if allow == nil || !allow(e.line) {
				fail("an unexpected line arrived after the close: %q", e.line)
			}
		case <-deadline:
			fail("the stream did not end within %s of the close", endWait)
		}
	}
}

// dial sends m<k> at k times every after its start while that time is below total, and reads
// each echo before the next send. It then stays silent until total and gap have passed, sends
// one more line, reads its echo, closes its side, and checks the trailer.
func dial(url string, every, total, gap time.Duration) {
	if url == "" || every <= 0 || total <= 0 || gap < 0 {
		fail("dial needs -url, a positive -every and -total, and a -gap that is not negative")
	}
	start := time.Now()
	s := open(url, "")
	s.await(responseWait)
	fmt.Printf("proto %s\n", s.resp.Proto)

	sent := 0
	exchange := func() {
		line := "m" + strconv.Itoa(sent)
		s.send(line)
		got := s.next(echoWait, "the echo of message "+strconv.Itoa(sent))
		if got != "echo "+line {
			fail("message %d: the echo is %q", sent, got)
		}
		sent++
	}
	for k := 0; time.Duration(k)*every < total; k++ {
		time.Sleep(time.Until(start.Add(time.Duration(k) * every)))
		exchange()
	}
	time.Sleep(time.Until(start.Add(total + gap)))
	exchange()

	trailer := s.finish(nil)
	if trailer != strconv.Itoa(sent) {
		fail("the trailer is %q, not %d", trailer, sent)
	}
	fmt.Printf("ok stream lasted %s with %d messages each way and trailer %s\n",
		time.Since(start).Round(time.Second), sent, trailer)
}

// shaped holds one direction of a stream silent for total while the other keeps sending.
func shaped(url, shape string, total time.Duration) {
	if url == "" || total <= 0 {
		fail("shaped needs -url and a positive -total")
	}
	switch shape {
	case "quiet-up":
		quietUp(url, total)
	case "quiet-down":
		quietDown(url, total)
	default:
		fmt.Fprintln(os.Stderr, "Use -shape quiet-up or quiet-down.")
		os.Exit(2)
	}
}

// quietUp sends one line, stays silent and reads ticks for total, then sends m1, waits for
// its acknowledgement, closes its side and requires the trailer up-closed.
func quietUp(url string, total time.Duration) {
	start := time.Now()
	s := open(url, "quiet-up")
	s.send("m0")
	s.await(responseWait)

	ticks := 0
	isTick := func(line string) bool {
		if line != "tick "+strconv.Itoa(ticks+1) {
			return false
		}
		ticks++
		return true
	}
	end := start.Add(total)
	for time.Now().Before(end) {
		select {
		case e, ok := <-s.events:
			if !ok || e.err == io.EOF {
				fail("the stream ended after %s and %d ticks", time.Since(start).Round(time.Second), ticks)
			}
			if e.err != nil {
				fail("the stream broke after %s and %d ticks: %v", time.Since(start).Round(time.Second), ticks, e.err)
			}
			if !isTick(e.line) {
				fail("after %d ticks, an unexpected line arrived: %q", ticks, e.line)
			}
		case <-time.After(min(tickWait, time.Until(end))):
			if time.Now().Before(end) {
				fail("no tick arrived for %s after %d ticks", tickWait, ticks)
			}
		}
	}

	s.send("m1")
	deadline := time.Now().Add(answerWait)
	for {
		line := s.next(time.Until(deadline), "ack m1")
		if line == "ack m1" {
			break
		}
		if !isTick(line) {
			fail("after %d ticks, an unexpected line arrived instead of ack m1: %q", ticks, line)
		}
	}
	trailer := s.finish(isTick)
	if trailer != upClosed {
		fail("the trailer is %q, not %s", trailer, upClosed)
	}
	fmt.Printf("ok quiet-up lasted %s with %d ticks down and one line up, ack m1, trailer %s\n",
		time.Since(start).Round(time.Second), ticks, trailer)
}

// quietDown sends a line every two seconds for total and fails if any part of the response
// arrives meanwhile. It then sends mark, waits for the count while its side is still open,
// closes its side and requires the trailer down-closed.
func quietDown(url string, total time.Duration) {
	start := time.Now()
	s := open(url, "quiet-down")
	sent := 0
	for k := 0; time.Duration(k)*tickEvery < total; k++ {
		wait := time.Until(start.Add(time.Duration(k) * tickEvery))
		select {
		case result := <-s.response:
			if result.err != nil {
				fail("the stream broke after %s and %d lines: %v", time.Since(start).Round(time.Second), sent, result.err)
			}
			fail("the response began after %s and %d lines, before mark", time.Since(start).Round(time.Second), sent)
		case <-time.After(wait):
		}
		s.send("m" + strconv.Itoa(sent))
		sent++
	}
	select {
	case result := <-s.response:
		if result.err != nil {
			fail("the stream broke after %d lines: %v", sent, result.err)
		}
		fail("the response began after %d lines, before mark", sent)
	case <-time.After(time.Until(start.Add(total))):
	}

	s.send(markLine)
	s.await(answerWait)
	count := s.next(answerWait, "the count")
	if count != "count "+strconv.Itoa(sent) {
		fail("the count is %q, but %d lines were sent", count, sent)
	}
	trailer := s.finish(nil)
	if trailer != downClosed {
		fail("the trailer is %q, not %s", trailer, downClosed)
	}
	fmt.Printf("ok quiet-down lasted %s with %d lines up, none down before mark, %s, trailer %s\n",
		time.Since(start).Round(time.Second), sent, count, trailer)
}

// fail prints a not ok line and ends the program with status 1.
func fail(format string, args ...any) {
	fmt.Printf("not ok %s\n", fmt.Sprintf(format, args...))
	os.Exit(1)
}
