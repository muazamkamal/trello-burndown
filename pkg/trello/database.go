package trello

import (
	"log"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"
)

// Board contains data of a trello board.
type Board struct {
	ID              string `gorm:"primary_key"`
	Name            string
	DateStart       time.Time
	DateEnd         time.Time
	Cards           uint
	Points          float64
	CardsCompleted  uint
	PointsCompleted float64
	CardProgress    []CardProgress
	TargetProgress  []TargetProgress
}

// CardProgress represents the progress of a card.
type CardProgress struct {
	gorm.Model
	BoardID string
	Date    time.Time
	Points  float64
}

// TargetProgress represents the target/total cards.
type TargetProgress struct {
	gorm.Model
	BoardID string
	Date    time.Time
	Points  float64
}

// GetDatabase returns a sqlite3 database connection.
func GetDatabase() *gorm.DB {
	db, err := gorm.Open(viper.GetString("database.dialect"), viper.GetString("database.url"))
	if err != nil {
		log.Fatalln(err)
	}
	db.AutoMigrate(&Board{}, &CardProgress{}, &TargetProgress{})
	return db
}

func saveToDatabase(board Board, m map[string]float64, targets map[string]float64) {
	db := GetDatabase()
	defer db.Close()
	oldBoard := Board{}
	db.Where("id = ?", board.ID).First(&oldBoard)
	db.Model(oldBoard).Updates(&board)
	db.Unscoped().Where("board_id = ?", board.ID).Delete(CardProgress{})
	db.Unscoped().Where("board_id = ?", board.ID).Delete(TargetProgress{})

	sprintDays := getDatesBetween(oldBoard.DateStart, oldBoard.DateEnd)
	firstDay := sprintDays[0].Format("2006-01-02")

	// For completed points

	pointsInWeekend := 0.0

	// For activities before sprint started
	for mDate, mVal := range m {
		mDateAsTime, _ := time.Parse("2006-01-02", mDate)

		if mDateAsTime.Before(sprintDays[0]) {
			m[firstDay] += mVal
		}
	}

	for _, day := range sprintDays {
		dayString := day.Format("2006-01-02")
		completedPoints := m[dayString]

		// Storing weekend points to bring over to the next monday
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			pointsInWeekend += completedPoints

			// Skipping storing weekend points into database,
			// this will be brought forward to the next monday.
			continue
		}

		db.Save(&CardProgress{
			Date:    day,
			Points:  completedPoints + pointsInWeekend,
			BoardID: board.ID,
		})

		// Reset weekend points once monday starts.
		pointsInWeekend = 0
	}

	// For burn up charts (calculating targets/total points)

	pointsInWeekend = 0

	// For activities before sprint started
	for targetDate, targetVal := range targets {
		targetDateAsTime, _ := time.Parse("2006-01-02", targetDate)

		if targetDateAsTime.Before(sprintDays[0]) {
			targets[firstDay] += targetVal
		}
	}

	for _, day := range sprintDays {
		dayString := day.Format("2006-01-02")
		targetPoints := targets[dayString]

		// Storing weekend points to bring over to the next monday
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			pointsInWeekend += targetPoints

			// Skipping storing weekend points into database,
			// this will be brought forward to the next monday.
			continue
		}

		db.Save(&TargetProgress{
			Date:    day,
			Points:  targetPoints + pointsInWeekend,
			BoardID: board.ID,
		})

		// Reset weekend points once monday starts.
		pointsInWeekend = 0
	}
}

func getDatesBetween(start time.Time, end time.Time) []time.Time {
	delta := int(end.Sub(start).Hours())
	delta /= 24

	var dates []time.Time
	for index := 0; index <= delta; index++ {
		date, _ := time.Parse("2006-01-02", start.Format("2006-01-02"))
		date = date.Add(time.Hour * 24 * time.Duration(index))
		dates = append(dates, date)
	}

	return dates
}
