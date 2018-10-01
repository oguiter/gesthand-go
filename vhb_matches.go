// semaine;"num poule";competition;poule;J;le;horaire;"club rec";"club vis";"club hote";"arb1 designe";"arb2 designe";observateur;delegue;
// "code renc";"nom salle";"adresse salle";CP;Ville;colle;"Coul. Rec";"Coul. Gard. Rec";"Coul. Vis";"Coul. Gard. Vis";
//"Ent. Rec";"Tel Ent. Rec";"Corresp. Rec";"Tel Corresp. Rec";"Ent. Vis";"Tel Ent. Vis";"Corresp. Vis";"Tel Corresp. Vis";"Num rec";"Num vis"

// 2018-39;M610035151;"TEST 2 IGNORE";"Poule 14";2;29/09/2018;15:00:00;"VILLENEUVE HB";"FRONTIGNAN THB";"VILLENEUVE  HANDBALL";;;;;NACCQVW;"COLLEGE LES SALINS";"71 , chemin carrière poissonniere";34750;"VILLENEUVE LES MAGUELONE";"Colle lavable à l'eau uniquement";Bleu;Noir;;;"BOUSIGE THIBAUT";0688310781;"BOUSIGE THIBAUT";0688310781;;;;;6134078;6134029
// ---
// 0: (semaine) 2018-39					// 1: (num poule)M610035151			// 2: (Competition) coupe de france departementale masculine 2018/2019
// 3: (poule) Poule 14					// 4: (Journee) 2					// 5: (date) 29/09/2018
// 6: (horaire) 15:00:00				// 7: (recevante) VILLENEUVE HB		// 8: (visiteur) FRONTIGNAN THB
// 9: (hote) VILLENEUVE HANDBALL		// 10: (arb1)						// 11: (arb2)
// 12: (obs)							// 13: (delegue)					// 14: (code renc) 		NACCQVW
// 15: (nom salle) C OLLEGE LES SALINS	// 16: (adresse salle) 	71 , chemin carrière poissonniere
// 17: (code postal) 	34750			// 18: (ville) VILLENEUVE LES MAGUELONE
// 19: (colle) 	Colle lavable à l'eau uniquement 	// 20: (coul recv) Bleu
// 21: (coul vis) Noir					// 22: (coul gb rec)				// 23: (coul gb vis)
// 24: (ent rec) BOUSIGE THIBAUT		// 25: (tel ent rec) 0688310781		// 26: (corresp rec) BOUSIGE THIBAUT
// 27: (tel corres rec) 0688310781		// 28:	// 29:	// 30:	// 31:
// 32: (num rec)  6134078				// 33: (num visi) 6134029

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

var myDBGTraces = 0 // 0, 1, 2 ...
var myDBGReadOnlyMode = 0

// DEFINES
//var defVHBCalName = "VHB MATCHS"
var defVHBCalName = "primary"
var defSecretCreds = "vhb34_cal_creds.json"
var defUserToken = "token.json"
var mColorID = make(map[string]int)

// -----------------------------------
// Usage: ggWeekStart
// fmt.Println(gWeekStart(2018, 1))
//		==> 2018-01-01 00:00:00 +0000 UTC
func gWeekStart(year, week int, loc *time.Location) time.Time {
	// Start from the middle of the year:
	t := time.Date(year, 7, 1, 0, 0, 0, 0, loc)

	// Roll back to Monday:
	if wd := t.Weekday(); wd == time.Sunday {
		t = t.AddDate(0, 0, -6)
	} else {
		t = t.AddDate(0, 0, -int(wd)+1)
	}

	// Difference in weeks:
	_, w := t.ISOWeek()
	t = t.AddDate(0, 0, (week-w)*7)

	return t
}

// ----------------------------------------
// fmt.Println(gWeekRange(2018, 1))
//		2018-01-01 00:00:00 +0000 UTC 2018-01-07 00:00:00 +0000 UTC
func gWeekRange(year, week int) (start, end time.Time) {
	start = gWeekStart(year, week, time.UTC)
	end = start.AddDate(0, 0, 6)
	return
}

// --------------------------------------------------------------------
//
func gPrepareEvent(srv *calendar.Service, strData []string, uCalID string) {

	if myDBGTraces > 2 {
		i := 0
		for v := range strData {
			_ = v
			fmt.Printf("%d: %s\n", i, strData[v])
			i++
		}
		fmt.Printf("---------------------------------------\n")
	}

	fmt.Printf("\n==> Prepare Match [%s]\n", strData[2])

	//== Normalization du titre
	compet := strings.ToLower(strData[2])
	compet = strings.Replace(compet, "test", "T", -1)

	compet = strings.Replace(compet, "masculine", "M", -1)
	compet = strings.Replace(compet, "masculin", "M", -1)
	compet = strings.Replace(compet, "feminine", "F", -1)
	compet = strings.Replace(compet, "feminin", "F", -1)
	compet = strings.Replace(compet, "championnat", "", -1)
	compet = strings.Replace(compet, "regional", "Reg.", -1)
	compet = strings.Replace(compet, "honneur", "Hon.", -1)
	compet = strings.Replace(compet, "territorial", "Ter.", -1)
	compet = strings.Replace(compet, "competition", "Comp.", -1)

	summ := fmt.Sprintf("[%s] %s/%s", compet, strData[7], strData[8])
	loc := fmt.Sprintf("%s, %s, %s %s", strData[15], strData[16], strData[17], strData[18])
	desc := fmt.Sprintf("J%s %s", strData[4], strData[2])

	//== Calcule des Dates de debut et fin
	// Si il n'y  pas de date indiquée , on va prendre le samedi de la semaine concernée
	// et on prévoit un horaire de 8:00 du matin :)
	// Durée du créneau: 1h30
	debDate := ""
	endDate := ""
	deb := time.Now()
	hTimeZone := "Europe/Paris"
	locTZ, _ := time.LoadLocation(hTimeZone)
	remind := 1 // Reminder

	if len(strData[5]) == 0 {
		// pas de date, juste une semaine... on va essayer de deviner :)
		// eg: week: "2018-39"
		wYear := 0
		wDay := 0
		fmt.Sscanf(strData[0], "%d-%d", &wYear, &wDay)

		Monday := gWeekStart(wYear, wDay, locTZ)
		Saturday := Monday.AddDate(0, 0, 5)
		deb = Saturday.Add(time.Hour * 8)

		// change few items
		summ = fmt.Sprintf("%s: %s/%s  !!! HORAIRE PAS ENCORE VALIDE !!!", compet, strData[7], strData[8])
		// Par defaut en Go, valeurs à false ...donc les Reminders ne sont pas actifs
		remind = 0
		//event['reminders']['useDefault'] = False;
	} else {
		//There's a date
		when := fmt.Sprintf("%s %s", strData[5], strData[6])
		layout := "02/01/2006 15:04:05" // Expected format
		debx, err := time.Parse(layout, when)
		_ = err
		deb = debx
	}

	// Construction des dates de debut et fin:
	words := strings.Fields(fmt.Sprintf("%s", deb))
	debDate = fmt.Sprintf("%sT%s", words[0], words[1])

	end := deb.Add(time.Minute * 90) //1h30
	words = strings.Fields(fmt.Sprintf("%s", end))
	endDate = fmt.Sprintf("%sT%s", words[0], words[1])

	fmt.Printf("\t==> Horaire: %s / %s\n", debDate, endDate)

	//== Color
	//## TODO To fix : modulo  12 for colorid !!!!

	// Si la poule n'existe pas, on ajout une entree dans la map
	if mColorID[strData[3]] == 0 {
		mColorID[strData[3]] = len(mColorID)
	}

	//try:
	//    event['colorId'] = team_list.index(lines['poule'])
	//
	//except ValueError:
	//    team_list.append(lines['poule'])

	//#start my colors from 4
	//colorId := (2 + team_list.index(lines['poule'])) % 12
	//colorId := 4

	strColor := fmt.Sprintf("%d", (2+mColorID[strData[3]])%12)
	//fmt.Println("ColoID: ", strColor)

	//== Tag
	// C'est le marqueur UNIQUE -
	// ATT: si l'entrée est supprimée, on peut la restaurer... par contre si elle est
	// enlevée de la corbeille, le tag continue à exister ==> 403 forbidden
	tag := fmt.Sprintf("a%s%s", strData[1], strData[4]) //  lines['num poule']+lines['J']
	id := strings.ToLower(tag)

	// TODO
	//fix 'id' to be RFC base32 compilant. Should avoid err 400
	// for ch in ['v','w', 'x','y','z']:
	//   if ch in event['id']:
	//     event['id']=event['id'].replace(ch,"p")
	id = strings.Replace(id, "v", "p", -1)
	id = strings.Replace(id, "w", "p", -1)
	id = strings.Replace(id, "x", "p", -1)
	id = strings.Replace(id, "y", "p", -1)
	id = strings.Replace(id, "z", "p", -1)

	// Event construction
	eventX := &calendar.Event{
		Id:          id, // Is UNIQUE !!!
		Summary:     summ,
		Location:    loc,
		Description: desc,
		ColorId:     strColor,

		Start: &calendar.EventDateTime{DateTime: debDate, TimeZone: hTimeZone},
		End:   &calendar.EventDateTime{DateTime: endDate, TimeZone: hTimeZone},
	}

	// Utilisation du reminder par defaut...
	/* TODO attendion sur le creneau de 8h00 du matin... */
	if remind == 1 {
		eventX.Reminders = &calendar.EventReminders{
			Overrides: []*calendar.EventReminder{
				{Method: "popup", Minutes: 60},
			},
			UseDefault:      false,
			ForceSendFields: []string{"UseDefault"},
		}
	}

	if myDBGTraces > 1 {
		fmt.Printf("+++++++++++ DBG Event +++++++++++\n")
		//TODO Vrai DUMP
		fmt.Printf("\tID: %s\n", id)
		fmt.Printf("\n\tSummary: %s\n\tloc: %s\n\tdesc: %s \n\ttag: %s \n", summ, loc, desc, tag)
		fmt.Printf("\tTZ: %s\n", hTimeZone)

		fmt.Printf("\tHORAIRES  :! deb [%s] / [%s]\n", debDate, endDate)
		fmt.Printf("\n+++++++++++++++++++++++++++++\n")
	}

	if myDBGReadOnlyMode == 0 {
		time.Sleep(500 * time.Millisecond)
		// Ecriture
		event, err := srv.Events.Insert(uCalID, eventX).Do()
		if err != nil {
			// An error occured:
			switch err.(*googleapi.Error).Code {
			case 400: // TODO
				fmt.Printf("%v\n", err)
				fmt.Println("WARNING TODO  ========= (400) IGNORE / PLATEAU TO FIX !!!")

			case 403: //TODO
				fmt.Printf("%v\n", err)
				fmt.Println(" WARNING TODO ======= Time out!!!")

			case 409: //  #already exist
				fmt.Printf("Warning 409: Try to update event %s...", eventX.Id)
				event, err = srv.Events.Update(uCalID, id, eventX).Do()
				_ = event
				if err == nil {
					fmt.Printf("(%s): SUCCESS\n", event.Id)
				} else {
					fmt.Printf("FAIL Result  [%v]\n", err)
				}
			default:
				log.Fatalf("Unable to create event. %v\n", err)
			}
		} else {
			fmt.Printf("Event created: OK %s\n", eventX.HtmlLink)
		}
	} else {
		fmt.Printf("WARNING / Readonly mode:%s\n", eventX.Description)
	}
}

//--------------------------------------------------------------------
func gProcessCSVFile(srv *calendar.Service, InputCSVfile string, uCalID string) {

	fmt.Printf("\n***** Traitement du fichier CSV: [%s] *****\n", InputCSVfile)

	fileIn, err := os.Open(InputCSVfile)
	if err != nil {
		log.Fatal(err)
	}
	defer fileIn.Close()
	skipHeaderDone := 0
	scanner := bufio.NewScanner(fileIn)
	for scanner.Scan() {
		testString := scanner.Text()
		if myDBGTraces > 1 {
			fmt.Println(testString)
			fmt.Println("---")
		}
		testArray := strings.Split(testString, ";")
		i := 0
		for v := range testArray {
			//Clean the datas/remove ""
			if len(testArray[v]) > 0 && testArray[v][0] == '"' {
				testArray[v] = testArray[v][1:]
			}
			if len(testArray[v]) > 0 && testArray[v][len(testArray[v])-1] == '"' {
				testArray[v] = testArray[v][:len(testArray[v])-1]
			}
			i++
		}

		//process to create the event but skip the header
		if skipHeaderDone > 0 {
			gPrepareEvent(srv, testArray, uCalID)
		}
		skipHeaderDone++
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

//----------------------------------------------------------------
// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := defUserToken
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

//----------------------------------------------------------------
// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	//	fmt.Printf("Go to the following link in your browser then type the "+
	//		"authorization code: \n%v\n", authURL)
	fmt.Printf("Merci d'ouvrir le lien suivant dans votre navigateur puis de saisir ci dessous le code d'autorisation indiqué: \n%v\n", authURL)

	var authCode string
	fmt.Printf("\nVotre code ?:")
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

//----------------------------------------------------------------
// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

//----------------------------------------------------------------
// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

// ----------------------------------------------------------------------
// gListUpcomingEvents lists all event, starting at a specific date/time
// ----------------------------------------------------------------------
func gListUpcomingEvents(srv *calendar.Service, uCalID string, showdeleted bool) {

	//t := time.Now().Format(time.RFC3339)
	t := "2016-09-01T11:59:50+02:00"

	if myDBGTraces > 1 {
		fmt.Printf("== DBG == gListUpcomingEvents for Cal ID: %s starting @ %s\n", uCalID, t)
	}

	events, err := srv.Events.List(uCalID).ShowDeleted(showdeleted).
		SingleEvents(true).TimeMin(t).MaxResults(200).OrderBy("startTime").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
	}

	//Affiche les evenements à venir
	fmt.Printf("\nEvenements enregistrés dans le calendrier depuis le %s:\n", t)
	if len(events.Items) == 0 {
		fmt.Println("No upcoming events found.")
	} else {
		for _, item := range events.Items {
			date := item.Start.DateTime
			if date == "" {
				date = item.Start.Date
			}
			fmt.Printf("ID: [%s] - (%s) - ", item.Id, item.Status)
			fmt.Printf("%v (%v)\n", item.Summary, date)
		}
	}
}

// ----------------------------------------------------------------------
// gGetCalendarID returns the ID of a wanted calendar
// ----------------------------------------------------------------------
func gGetCalendarID(srv *calendar.Service, calendarName string) string {

	calList, err := srv.CalendarList.List().Do()
	if err != nil {
		log.Fatalf("Unable to retrieve the Calendar list: %v", err)
	}

	if len(calList.Items) == 0 {
		fmt.Println("No calendar found. SHOULD NOT HAPPEND")
	} else {
		for _, iCal := range calList.Items {
			if myDBGTraces > 1 {
				fmt.Printf("ID: [%s] - (%s) \n", iCal.Id, iCal.Summary)
			}
			if strings.Compare(calendarName, iCal.Summary) == 0 {
				return iCal.Id
			}
		}
	}
	return ""
}

// ----------------------------------------------------------------------
// ----------------------------------------------------------------------
func main() {
	fmt.Println("===========================================================")
	fmt.Println(" Gesthand extraction / Google calendar v1.1.0				")
	fmt.Println("===========================================================")

	InputFileF := flag.String("csv", "gesthand_test.csv", "Input file")
	verbose := flag.Bool("verbose", false, "verbose")
	listEvent := flag.Bool("list", false, "Liste upcoming event")

	flag.Parse()
	myDBGTraces = 0

	if *verbose == true {
		myDBGTraces = 2
		fmt.Println("(Verbose mode)")
	}
	// Get input file
	/*	 := "gesthand_test.csv" // default file name
	if len(os.Args) > 1 {
		InputFile = os.Args[1] // Get the file name
	}
	*/
	fmt.Printf("Input CSV file : [%s]\n", *InputFileF)

	// Get the TOKEN
	b, err := ioutil.ReadFile(defSecretCreds)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	// Readonly config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	// Now, create the serv ref
	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	// List and Find calendar
	//NOT USED gGetCalendarsList(srv)
	mCalendarID := "primary"
	if strings.Compare(defVHBCalName, "primary") != 0 {
		mCalendarID = gGetCalendarID(srv, defVHBCalName)
	}

	if len(mCalendarID) == 0 {
		log.Fatalf("Unable to retrieve Calendar ID for calname:%s \n%v", defVHBCalName, err)
	}
	if myDBGTraces > 1 {
		fmt.Printf("CalID: %s\n", mCalendarID)
	}

	if *listEvent == true {
		// List all upcoming events for a given agenda
		fmt.Println("Upcoming event deleted")
		gListUpcomingEvents(srv, mCalendarID, true)
	} else {
		gProcessCSVFile(srv, *InputFileF, mCalendarID)
	}

}
