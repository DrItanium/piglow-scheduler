package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

var channel0 = flag.String("chan0", "", "path to channel 0")
var channel1 = flag.String("chan1", "", "path to channel 1")
var channel2 = flag.String("chan2", "", "path to channel 2")

type µop [7]byte

func (this µop) Delay() byte {
	return this[6]
}

type piglow_µop [19]byte

func (this piglow_µop) Delay() byte {
	return this[18]
}
func (this piglow_µop) SetLeg(index int, leg µop) error {
	var begin, end int
	var slice []byte
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
	slice = this[begin:end]
	for index, _ := range slice {
		slice[index] = leg[index]
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
func main() {
	flag.Parse()
	if e := checkArgs(); e != nil {
		panic(e)
	}
	// read in three µop chains at a time
	var f0, f1, f2 *os.File
	var err error
	if f0, err = os.Open(*channel0); err != nil {
		fmt.Println(err)
		return
	}
	defer f0.Close()
	if f1, err = os.Open(*channel1); err != nil {
		fmt.Println(err)
		return
	}
	defer f1.Close()
	if f2, err = os.Open(*channel2); err != nil {
		fmt.Println(err)
		return
	}
	defer f2.Close()
	// we can now open up our input sources
	in0, err0 := definput(f0)
	in1, err1 := definput(f1)
	in2, err2 := definput(f2)
	var done [3]bool
	var contents [3]µop
	//var op piglow_µop
	trySet := func(index int, val µop) {
		if !done[index] {
			contents[index] = val
		}
	}
	for {
		if done[0] && done[1] && done[2] {
			break
		}
		select {
		case d0 := <-err0:
			if d0 == nil {
				done[0] = true
			} else {
				panic(d0)
			}
		case d1 := <-err1:
			if d1 == nil {
				done[1] = true
			} else {
				panic(d1)
			}
		case d2 := <-err2:
			if d2 == nil {
				done[2] = true
			} else {
				panic(d2)
			}
		default:
			trySet(0, <-in0)
			trySet(1, <-in1)
			trySet(2, <-in2)
		}
	}
}
func definput(input io.Reader) (chan µop, chan error) {
	output := make(chan µop)
	finished := make(chan error)
	go func(i io.Reader, o chan µop, d chan error) {
		q := bufio.NewReader(i)
		container := make([]byte, 7)
		var count int
		var err error
		for count, err = q.Read(container); err == nil && count == 7; count, err = q.Read(container) {
			o <- µop{
				container[0],
				container[1],
				container[2],
				container[3],
				container[4],
				container[5],
				container[6],
			}
		}
		if err != nil && err != io.EOF {
			d <- err
		} else if err == nil && count != 7 {
			d <- fmt.Errorf("Was unable to read %d bytes!", 7)
		}
		d <- nil
		close(o)
		close(d)
	}(input, output, finished)
	return output, finished
}
