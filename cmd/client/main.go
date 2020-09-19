package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/olekukonko/tablewriter"
	"github.com/tanelmae/grpc-sample/pb"
	"google.golang.org/grpc"
)

const (
	simpleDateFormat = "2006-01-02"

	formatJSON   = "json"
	formatSilent = "silent"
	formatTable  = "table"
)

type cmdFlags struct {
	name       string
	flagSet    *flag.FlagSet
	serverAddr *string
	output     *string
	from       *string
	to         *string
	secondFrom *string
	secondTo   *string
	maxRows    *int
	maxColumns *int
}

func (cmd cmdFlags) Parse() {
	cmd.flagSet.Parse(os.Args[2:])
}

func (cmd cmdFlags) Print() {
	cmd.flagSet.PrintDefaults()
}

func newCmd(name string) cmdFlags {
	flagSet := flag.NewFlagSet(name, flag.ExitOnError)
	return cmdFlags{
		flagSet:    flagSet,
		name:       name,
		serverAddr: flagSet.String("addr", "localhost:8080", "Server address"),
		output:     flagSet.String("out", "", "Format for the command output"),
		from:       flagSet.String("from", "2019-03-01", "Start time for the period"),
		to:         flagSet.String("to", "2019-04-01", "End time for the period"),
	}
}

func main() {
	// rpc CategoryScores(TimePeriod) returns (CategoryScoresOut)
	categoryScoresCmd := newCmd("category-scores")
	categoryScoresCmd.maxColumns = categoryScoresCmd.flagSet.Int("max-cols", 5, "Max columns for the table output")
	// rpc TicketScores(TimePeriod) returns (TicketScoresOut)
	ticketScoresCmd := newCmd("ticket-scores")
	ticketScoresCmd.maxRows = ticketScoresCmd.flagSet.Int("max-rows", 5, "Max rows for the table output")
	// rpc OveralScore(TimePeriod) returns (OveralScoreOut);
	overallScoresCmd := newCmd("overall-score")
	// rpc PeriodOverPeriod(TimePeriods) returns (PeriodOut);
	diffCmd := newCmd("period-diff")
	diffCmd.secondFrom = diffCmd.flagSet.String("second-from", "2019-04-01", "Start time for the period")
	diffCmd.secondTo = diffCmd.flagSet.String("second-to", "2019-04-30", "End time for the period")

	flag.Usage = func() {
		fmt.Printf("Supported subcommands and flags:\n\n")
		fmt.Printf("%s\n", categoryScoresCmd.name)
		categoryScoresCmd.Print()
		fmt.Printf("\n%s\n", ticketScoresCmd.name)
		ticketScoresCmd.Print()
		fmt.Printf("\n%s\n", overallScoresCmd.name)
		overallScoresCmd.Print()
		fmt.Printf("\n%s\n", diffCmd.name)
		diffCmd.Print()
	}

	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	switch os.Args[1] {
	case categoryScoresCmd.name:
		categoryScoresCmd.Parse()

		reqFrom, err := protoTime(*categoryScoresCmd.from)
		if err != nil {
			panic(err)
		}

		reqTo, err := protoTime(*categoryScoresCmd.to)
		if err != nil {
			panic(err)
		}

		conn, err := grpc.Dial(*categoryScoresCmd.serverAddr, grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		client := pb.NewTicketServiceClient(conn)

		resp, err := client.CategoryScores(ctx, &pb.TimePeriod{
			From: reqFrom,
			To:   reqTo,
		})

		if err != nil {
			panic(err)
		}

		switch *categoryScoresCmd.output {
		case formatJSON:
			b, err := json.MarshalIndent(resp, "", "    ")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("%s\n", string(b))
		case formatTable:
			header := []string{"Category", "Ratings"}
			tableData := [][]string{}

			for _, category := range resp.Counts {
				tableData = append(tableData, []string{category.Name, fmt.Sprint(category.Count)})
			}

			periods := []string{}
			dataMap := map[string]map[string]int32{}
			for _, dataPoint := range resp.Scores {
				var dataCol map[string]int32
				var ok bool
				if dataCol, ok = dataMap[dataPoint.Period]; !ok {
					dataCol = map[string]int32{}
					periods = append(periods, dataPoint.Period)
				}
				dataCol[dataPoint.Category] = dataPoint.Score
				dataMap[dataPoint.Period] = dataCol
			}

			for counter, colName := range periods {
				if counter > *ticketScoresCmd.maxRows {
					break
				}
				header = append(header, colName)
				for index, category := range resp.Counts {
					cellVal := fmt.Sprintf("%d %%", dataMap[colName][category.Name])
					tableData[index] = append(tableData[index], cellVal)
				}
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader(header)
			table.AppendBulk(tableData)
			table.Render()
			fmt.Printf("Output limited to %d columns. Use -max-cols flag to change that\n",
				*categoryScoresCmd.maxColumns)
		case formatSilent:
			fmt.Println("output omitted")
		default:
			log.Printf("%+v", resp)
		}
	case ticketScoresCmd.name:
		ticketScoresCmd.Parse()
		reqFrom, err := protoTime(*ticketScoresCmd.from)
		if err != nil {
			panic(err)
		}

		reqTo, err := protoTime(*ticketScoresCmd.to)
		if err != nil {
			panic(err)
		}

		conn, err := grpc.Dial(*ticketScoresCmd.serverAddr, grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		client := pb.NewTicketServiceClient(conn)

		resp, err := client.TicketScores(ctx, &pb.TimePeriod{
			From: reqFrom,
			To:   reqTo,
		})

		if err != nil {
			panic(err)
		}
		switch *ticketScoresCmd.output {
		case formatJSON:
			b, err := json.MarshalIndent(resp, "", "    ")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("%s\n", string(b))
		case formatSilent:
			fmt.Println("output omitted")
		case formatTable:
			table := tablewriter.NewWriter(os.Stdout)
			header := append([]string{"Ticket"}, resp.Categories...)
			dataTable := [][]string{}

			dataMap := map[int32]map[string]int32{}

			for _, dataPoint := range resp.Scores {
				var dataRow map[string]int32
				var ok bool
				if dataRow, ok = dataMap[dataPoint.Id]; !ok {
					dataRow = map[string]int32{"Ticket": dataPoint.Id}
				}
				dataRow[dataPoint.Category] = dataPoint.Score
				dataMap[dataPoint.Id] = dataRow
			}

			counter := 0
			for _, dataRow := range dataMap {
				if counter > *ticketScoresCmd.maxRows {
					break
				}
				dataItems := []string{}
				for _, category := range header {
					dataItems = append(dataItems, fmt.Sprintf("%d %%", dataRow[category]))
				}
				dataTable = append(dataTable, dataItems)
				counter++
			}

			table.SetHeader(header)
			table.AppendBulk(dataTable)
			table.Render()

			fmt.Printf("Output limited to %d rows. Use -max-rows flag to change that\n",
				*ticketScoresCmd.maxRows)
		default:
			log.Printf("%+v", resp)
		}
	case overallScoresCmd.name:
		overallScoresCmd.Parse()
		reqFrom, err := protoTime(*overallScoresCmd.from)
		if err != nil {
			panic(err)
		}

		reqTo, err := protoTime(*overallScoresCmd.to)
		if err != nil {
			panic(err)
		}

		conn, err := grpc.Dial(*overallScoresCmd.serverAddr, grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		client := pb.NewTicketServiceClient(conn)

		resp, err := client.OveralScore(ctx, &pb.TimePeriod{
			From: reqFrom,
			To:   reqTo,
		})

		if err != nil {
			panic(err)
		}

		switch *overallScoresCmd.output {
		case formatJSON:
			b, err := json.MarshalIndent(resp, "", "    ")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("%s\n", string(b))
		case formatSilent:
			fmt.Println("output omitted")
		case formatTable:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Overall score"})
			table.Append([]string{fmt.Sprintf("%d %%", resp.Score)})
			table.Render()
		default:
			log.Printf("%+v", resp)
		}
	case diffCmd.name:
		diffCmd.Parse()
		reqFirstFrom, err := protoTime(*diffCmd.from)
		if err != nil {
			panic(err)
		}

		reqFirstTo, err := protoTime(*diffCmd.to)
		if err != nil {
			panic(err)
		}

		reqSecondFrom, err := protoTime(*diffCmd.secondFrom)
		if err != nil {
			panic(err)
		}

		reqSecondTo, err := protoTime(*diffCmd.secondTo)
		if err != nil {
			panic(err)
		}

		conn, err := grpc.Dial(*diffCmd.serverAddr, grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		client := pb.NewTicketServiceClient(conn)

		resp, err := client.PeriodOverPeriod(ctx, &pb.TimePeriods{
			First: &pb.TimePeriod{
				From: reqFirstFrom,
				To:   reqFirstTo,
			},
			Second: &pb.TimePeriod{
				From: reqSecondFrom,
				To:   reqSecondTo,
			},
		})

		if err != nil {
			panic(err)
		}

		switch *diffCmd.output {
		case formatJSON:
			b, err := json.MarshalIndent(resp, "", "    ")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("%s\n", string(b))
		case formatSilent:
			fmt.Println("output omitted")
		case formatTable:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Category", "Change"})
			for _, change := range resp.Changes {
				table.Append([]string{change.Category, fmt.Sprintf("%d %%", change.Diff)})
			}
			table.Render()
		default:
			log.Printf("%+v", resp)
		}
	default:
		fmt.Printf("Unknown subcommand \"%s\"\n\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
	}

}

func protoTime(timeString string) (*timestamp.Timestamp, error) {
	goTime, err := time.Parse(simpleDateFormat, timeString)
	if err != nil {
		return nil, err
	}

	pbTime, err := ptypes.TimestampProto(goTime)
	if err != nil {
		return nil, err
	}
	return pbTime, nil
}
