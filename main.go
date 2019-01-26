package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var address = "62.109.18.74"

var (
	err       error
	DIR       = "files" // полный путь к папке
	ListFiles []string  // массив строк с названием файлом. можно сделать карту и удалять по ключу и так же хранить больше информации
)

type Data struct {
	X float64 `json: "X"`
	Y float64 `json: "Y"`
}

func ParseFile(path string) (result []Data) {
	r := regexp.MustCompile("[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?")

	file, err := os.Open(path)
	if err != nil {
		log.Println(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := r.FindAllString(scanner.Text(), -1)
		if len(f) == 2 {
			if x, err := strconv.ParseFloat(f[0], 64); err == nil {
				if y, err := strconv.ParseFloat(f[1], 64); err == nil {
					result = append(result, Data{X: x, Y: y})
				}
			}
		}
	}
	return result
}

// обновление списка файлов
func GetListFiles(dir string) {
	// получение списка всех файлов и папок в нашей деректории
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	// очискта массива
	ListFiles = ListFiles[:0]
	// перебор всех файлов и их фильтр
	for _, file := range files {
		// по расширению .txt
		if filepath.Ext(file.Name()) == ".txt" {
			// добавление файла в масссив
			ListFiles = append(ListFiles, file.Name())
		}
	}
}

// генерация главной страницы
func viewFiles(w http.ResponseWriter, r *http.Request) {
	// получение/обновление списка файлов
	GetListFiles(DIR)
	// настройка заголовока, что это html документ
	w.Header().Set("Content-Type", "text/html")
	// создание html шаблона со списком файлов
	HtmlListFiles := "<body/><p>BIOCAD's DataView 1.0 - Data Visualization Service</p><table>"
	for i, f := range ListFiles {
		HtmlListFiles = HtmlListFiles + "<tr><td>" + strconv.Itoa(i+1) + "</td><td>" + f + "</td><td><a href='/visualisation/" + f + "'>Visualise</a></td><td><a href='/delete/'" + f + "'>Delete</a></tr>"
	}
	HtmlListFiles = HtmlListFiles + "</table>"
	// вывод списка файлов и форм добавления
	io.WriteString(w, HtmlListFiles+`
	  <form method="POST" enctype="multipart/form-data" action="/add">
		<input type="file" name="file">
		<input type="submit">
	  </form>
	  <p>If you have any questions - contact me at: bosh.anastasia@gmail.com</p><p>(c) 2019</p>
	  </body>`)
}

// удаление файла
func deleteFile(w http.ResponseWriter, r *http.Request) {
	// получение название файла из URL
	FileName := r.URL.Path[len("/delete/"):]
	// полный путь к файлу
	fullDstFilePath := DIR + "\\" + FileName
	// проверяем наличие файла
	if _, err := os.Stat(fullDstFilePath); err == nil {
		// удаление
		if err := os.Remove(fullDstFilePath); err != nil {
			log.Println(err)
		}
	}
	// редирект на главную страницу
	http.Redirect(w, r, address, 307)
}

// добавления файла
func addFile(w http.ResponseWriter, r *http.Request) {
	// обработка только POST запрос
	if r.Method == "POST" {
		// получение файла из формы
		// src - временный файл
		// fileLoad - заголовки файла
		src, fileLoad, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer src.Close()
		// проверка типа файла по заголовку
		if fileLoad.Header.Get("Content-Type") == "text/plain" {
			// создание файла с таким же названием в нашей папке
			dst, err := os.Create(filepath.Join(DIR, fileLoad.Filename))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			defer dst.Close()
			// копирование содержимого файла
			io.Copy(dst, src)

		}
	}
	// редирект на главную страницу
	http.Redirect(w, r, "/", 307)
	time.Sleep(time.Second * 3)
}

// построение диаграммы
func getView(w http.ResponseWriter, r *http.Request) {
	// получение название файла из URL
	FileName := r.URL.Path[len("/visualisation/"):]
	fullDstFilePath := DIR + "/" + FileName

	// генерация страницы при помощи сервиса Google Charts
	var HtmlListFiles string
	for _, p := range ParseFile(fullDstFilePath) {
		HtmlListFiles = HtmlListFiles + "[" + fmt.Sprintf("%f", p.X) + "," + fmt.Sprintf("%f", p.Y) + "],"
	}

	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, `
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
    <script type="text/javascript">
    google.charts.load('current', {packages: ['corechart', 'line']});
google.charts.setOnLoadCallback(drawChart);

      function drawChart() {
		var data = new google.visualization.DataTable();
		data.addColumn('number', 'time');
		data.addColumn('number', 'measurment');
		data.addRows([
			`+HtmlListFiles+`
        ]);

        var options = {
			width: 860,
			height: 300
		  };

        var chart = new google.visualization.LineChart(document.getElementById('curve_chart'));

        chart.draw(data, options);
      }
    </script>
  </head>
  <body>
  <p>BIOCAD's DataView 1.0 - Data Visualization Service</p>
  <div id="chart_wrapper" style=" overflow-x: scroll;overflow-y: hidden;width: 1100px;">
	<div id="curve_chart"></div>
	</div>
	<form action="/">
		<input type="submit" value="back">
	  </form>
	<p>If you have any questions - contact me at: bosh.anastasia@gmail.com</p><p>(c) 2019</p>
  </body>`)
}

func main() {
	http.HandleFunc("/add", addFile)
	http.HandleFunc("/delete/", deleteFile)
	http.HandleFunc("/", viewFiles)
	http.HandleFunc("/visualisation/", getView)
	// запуск сервера
	http.ListenAndServe(address, nil)
}
