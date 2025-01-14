package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"

	"github.com/frankh/nano/address"
	"github.com/frankh/nano/types"
	"github.com/urfave/cli"
)

var (
	iterations float64
)

func main() {
	app := cli.NewApp()
	app.Name = "Nano Vanity Generator"
	app.Usage = "Generate wallet seeds with desirable public addresses"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "prefixes, p",
			Usage: "Prefixes to search for at the start of address, comma separated",
		},
		cli.IntFlag{
			Name:  "count, n",
			Value: 1,
			Usage: "Number of valid addresses to generate before exiting, or 0 for infinite (default=1).",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Do not output progress message.",
		},
	}
	app.Action = func(c *cli.Context) {

		longestPrefix := ""
		prefixes := strings.Split(c.String("prefixes"), ",")
		for i, prefix := range prefixes {
			prefixes[i] = strings.TrimSpace(prefix)
			if len(prefix) > len(longestPrefix) {
				longestPrefix = prefix
			}
		}
		iterations = estimatedIterations(longestPrefix)
		quiet := c.Bool("quiet")

		if !quiet {
			fmt.Println("Estimated number of iterations needed:", int(iterations))
		}
		for i := 0; i < c.Int("count") || c.Int("count") == 0; i++ {
			seed, addr, err := generateVanityAddress(prefixes, quiet)
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			fmt.Printf(`Found matching address!
Seed: %s
Address: %s

`, strings.ToUpper(seed), addr)
		}
	}
	app.Run(os.Args)
}

func estimatedIterations(prefix string) float64 {
	return math.Pow(32, float64(len(prefix))) / 2
}

func isValidPrefix(prefix string) bool {
	for _, c := range prefix {
		if !strings.Contains(address.EncodeNano, string(c)) {
			return false
		}
	}
	return true
}

func generateVanityAddress(prefixes []string, quiet bool) (string, types.Account, error) {
	for _, prefix := range prefixes {
		if !isValidPrefix(prefix) {
			return "", "", fmt.Errorf("Invalid character in prefix " + prefix)
		}
	}

	c := make(chan string, 100)
	progress := make(chan int, 100)

	for i := 0; i < runtime.NumCPU(); i++ {
		go func(c chan string, progress chan int) {
			defer func() {
				recover()
			}()
			count := 0
			for {
				count++
				if count%(500+i) == 0 {
					progress <- count
					count = 0
				}
				seedBytes := make([]byte, 32)
				_, err := rand.Read(seedBytes)
				if err != nil {
					panic("Failed to generate random seed")
				}

				seed := hex.EncodeToString(seedBytes)
				pub, _ := address.KeypairFromSeed(seed, 0)
				address := string(address.PubKeyToAddress(pub))

				for _, prefix := range prefixes {
					if strings.HasPrefix(address[6:], prefix) {
						c <- seed
						break
					}
				}
			}
		}(c, progress)
	}

	go func(progress chan int) {
		total := 0
		fmt.Println()
		for {
			count, ok := <-progress
			if !ok {
				break
			}
			total += count
			if !quiet {
				fmt.Printf("\033[1A\033[KTried %d (~%.2f%%)\n", total, float64(total)/iterations*100)
			}
		}
	}(progress)

	seed := <-c
	pub, _ := address.KeypairFromSeed(seed, 0)
	account := address.PubKeyToAddress(pub)
	if !address.ValidateAddress(account) {
		return "", "", fmt.Errorf("Address generated had an invalid checksum!\nPlease create an issue on github: https://github.com/frankh/rai-vanity")
	}

	close(c)
	close(progress)

	return seed, account, nil
}
