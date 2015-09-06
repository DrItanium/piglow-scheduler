package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

var channel0 = flag.String("chan0", "", "path to channel 0 (required)")
var channel1 = flag.String("chan1", "", "path to channel 1 (required)")
var channel2 = flag.String("chan2", "", "path to channel 2 (required)")
var delay = flag.Uint("delay", 5, "delay to add to each µop before sending!")

const (
	LegDelay = 6
	µopDelay = 18
)

func SetLeg(index int, in, leg []byte) error {
	var begin, end int
	switch index {
	case 0:
		begin = 0
		end = 6
	case 1:
		begin = 6
		end = 12
	case 2:
		begin = 12
		end = 18
	default:
		return fmt.Errorf("Leg index %d is out of range!", index)
	}
	for i, j := begin, 0; i < end; i, j = i+1, j+1 {
		in[i] = leg[j]
	}
	return nil
}
func checkArgs() error {
	if *channel0 == "" {
		return fmt.Errorf("Channel0 not set")
	} else if *channel1 == "" {
		return fmt.Errorf("Channel1 not set")
	} else if *channel2 == "" {
		return fmt.Errorf("Channel2 not set")
	} else {
		return nil
	}
}

type Processor struct {
	path   string
	file   *os.File
	reader *bufio.Reader
	done   bool
	data   chan []byte
}

func (this *Processor) Close() error {
	return this.file.Close()
}
func (this *Processor) ProcessData() {
	buf := make([]byte, 7)
	var err error
	var count int
	for count, err = this.reader.Read(buf); err == nil; count, err = this.reader.Read(buf) {
		for i := count; i < len(buf); i++ {
			buf[i] = 0
		}
		c := make([]byte, len(buf))
		copy(c, buf)
		count := c[6] // needs to have at least one iteration
		c[6] = 0
		// need to have at least one iteration
		for i := 0; i < int(count+1); i++ {
			this.data <- c
		}
		// zero out the data now that we have made a copy
		for i := 0; i < len(buf); i++ {
			buf[i] = 0
		}
	}
	if err != io.EOF {
		panic(err)
	}
	close(this.data)
	this.done = true
}

func New(path string) (*Processor, error) {
	var proc Processor
	if f, err := os.Open(path); err != nil {
		return nil, err
	} else {
		proc.path = path
		proc.file = f
		proc.reader = bufio.NewReader(f)
		proc.done = false
		proc.data = make(chan []byte)
		return &proc, nil
	}
}

type Processors []*Processor

func BuildProcessors(inputs ...string) (Processors, error) {
	procs := make(Processors, len(inputs))
	for index, str := range inputs {
		if proc, err := New(str); err != nil {
			for _, proc := range procs {
				if proc != nil {
					proc.Close()
				}
			}
			return nil, err
		} else {
			go proc.ProcessData()
			procs[index] = proc
		}
	}
	return procs, nil
}
func (this Processors) Close() error {
	// we need to make sure that all processors are closed even if errors occurred during closing
	var msg string
	var errorsFound bool
	for _, p := range this {
		if err := p.Close(); err != nil {
			if !errorsFound {
				msg += fmt.Sprintf("\t- %s: %s", p.path, err)
				errorsFound = true
			} else {
				msg += fmt.Sprintf("\n\t- %s: %s", p.path, err)
			}
		}
	}
	if errorsFound {
		return fmt.Errorf("Errors happened during close:\n %s", msg)
	}
	return nil
}
func main() {
	flag.Parse()
	if e := checkArgs(); e != nil {
		fmt.Println(e)
		flag.Usage()
		return
	}
	// read in three µop chains at a time
	procs, err := BuildProcessors(*channel0, *channel1, *channel2)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer procs.Close()
	op := make([]byte, 6*len(procs)+1)
	op[len(op)-1] = byte(*delay)
	out := make(chan [][]byte)
	// at this point we have some processors waiting on us to do something
	go func(procs Processors) {
		// process the µops until we run out of elements
		last := make([][]byte, len(procs))
		for i := 0; i < len(procs); i++ {
			last[i] = make([]byte, 7)
		}
		for outcome := true; outcome; {
			outcome = false
			for ind, proc := range procs {
				if !proc.done {
					outcome = true
					dat := <-proc.data
					copy(last[ind], dat)
				}
			}
			out <- last
		}
		close(out)
	}(procs)
	for {
		if val, ok := <-out; !ok {
			break
		} else {
			for ind, a := range val {
				from, to := ind*6, (ind+1)*6
				// capture a slice and setup the op
				copy(op[from:to], a)
			}
			op[len(op)-1] = byte(*delay)
			opBuf := bytes.NewBuffer(op)
			if _, err := opBuf.WriteTo(os.Stdout); err != nil {
				panic(err)
			}
		}
	}
}
