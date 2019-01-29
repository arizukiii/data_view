package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"html/template"

	"github.com/gorilla/websocket"
)

//шаблон сообщения для сокета
type msg struct {
	Type    string
	Content string
	Info    string
}

//структура данных в файле
type Data struct {
	X float64 `json: "X"`
	Y float64 `json: "Y"`
}

var (
	//настройки сокета
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	err       error
	DIR       = ""     // полный путь к папке
	ListFiles []string // массив строк с названием файлом
)

func main() {
	//запись текущего положения файла в переменную DIR
	DIR, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(DIR)
	//создание маршрутов
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/", home)

	//запуск сервера
	panic(http.ListenAndServe(":8080", nil))
}

//главная страница
func home(w http.ResponseWriter, r *http.Request) {
	//отдаём HTML шаблон
	homeTemplate.Execute(w, "")
}

//слушаем вебсокет
func wsHandler(w http.ResponseWriter, r *http.Request) {
	//открывем соединение с настройками
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	//отложенная функция закрытия
	defer conn.Close()
	//слушаем текущее соединение в бесконечном цикле, пока не произойдёт обрыв
	for {
		//парсим полученное сообщение по сокету
		m := msg{}
		if err := conn.ReadJSON(&m); err != nil {
			fmt.Println("Error reading json.", err)
			break
		}
		//обробатываем сообщение по типу. сообщения не соответствующие нашему шаблону, выдадут ошибку и не будут обрабатываться
		switch m.Type {
		//запрос на получение актуальной таблицы
		case "getTable":
			//получаем таблицу файлов и отдаём назад
			r := msg{Type: "updateTable", Content: GetListFiles(DIR)}
			if err = conn.WriteJSON(r); err != nil {
				fmt.Println(err)
				break
			}
			//загрузка файла
		case "loadFile":
			//созданём файл с таким же именем, но пустой. и открываем его
			file, err := os.Create(DIR + "/" + m.Info)
			if err != nil {
				fmt.Println("Unable to create file:", err)
				os.Exit(1)
			}
			//отложенная функция закроет файл
			defer file.Close()
			//записываем содержимое файла
			file.WriteString(m.Content)
			//получаем таблицу файлов и отдаём назад
			r := msg{Type: "updateTable", Content: GetListFiles(DIR)}
			if err = conn.WriteJSON(r); err != nil {
				fmt.Println(err)
				break
			}
		case "deleteFile":
			//делаем полный путь к файлу
			fullDstFilePath := DIR + "/" + m.Info
			//проверяем наличие файла
			if _, err := os.Stat(fullDstFilePath); err == nil {
				//удаляем
				if err := os.Remove(fullDstFilePath); err != nil {
					log.Println(err)
				}
				//получаем таблицу файлов и отдаём назад
				r := msg{Type: "updateTable", Content: GetListFiles(DIR)}
				if err = conn.WriteJSON(r); err != nil {
					fmt.Println(err)
					break
				}
			}
		case "visualFile":
			//проверяем наличие файла
			fullDstFilePath := DIR + "/" + m.Info
			// получаю данные из файла и создаём шаблон для графика
			var HtmlListFiles string
			arr := ParseFile(fullDstFilePath)
			for i, p := range arr {
				HtmlListFiles = HtmlListFiles + "[" + fmt.Sprintf("%f", p.X) + "," + fmt.Sprintf("%f", p.Y) + "]"
				if len(arr)-1 > i {
					HtmlListFiles = HtmlListFiles + ","
				}
			}
			//отправляем клиенту с типом initChart и данным для графика
			r := msg{Type: "initChart", Content: HtmlListFiles}
			if err = conn.WriteJSON(r); err != nil {
				fmt.Println(err)
				break
			}
		}
	}
}

//функция обработки файла
func ParseFile(path string) (result []Data) {
	//создаём регулярное выражение соответствующее нашим треботваниям
	r := regexp.MustCompile("[-+]?[0-9]*\\.?[0-9]+([eE][-+]?[0-9]+)?")
	//открываем файл
	file, err := os.Open(path)
	if err != nil {
		log.Println(err)
	}
	//отложенная функция закроет файл
	defer file.Close()
	//записываем все строки в переменную scanner
	scanner := bufio.NewScanner(file)
	//обробатываем каждую строку
	for scanner.Scan() {
		//ищем в строке подстроки, соответствующие нашему регулярному выражению
		f := r.FindAllString(scanner.Text(), -1)
		//если подстроки ровно две - запишим
		if len(f) == 2 {
			//преобразуем подстроку в тип флоат и запишим как Х
			if x, err := strconv.ParseFloat(f[0], 64); err == nil {
				//преобразуем подстроку в тип флоат и запишим как У
				if y, err := strconv.ParseFloat(f[1], 64); err == nil {
					//данные добавим в массив
					result = append(result, Data{X: x, Y: y})
				}
			}
		}
	}
	return result
}

// обновляем список файлов
func GetListFiles(dir string) string {
	//получаем список всех файлов и папок в нашей деректории
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	//очищаем массив
	ListFiles = ListFiles[:0]
	//перебираем все файлы и фильруем
	for _, file := range files {
		//по расширению .txt
		if filepath.Ext(file.Name()) == ".txt" {
			//добавляем файл в масссив
			ListFiles = append(ListFiles, file.Name())
		}
	}
	//сформируем html код, для вывода на странице
	HtmlListFiles := "<table>"
	for i, f := range ListFiles {
		HtmlListFiles = HtmlListFiles + "<tr><td>" + strconv.Itoa(i+1) + "</td><td>" + f + "</td><td><button class='visual-file' data-name='" + f + "'>Visual</button></td><td><button class='delete-file' data-name='" + f + "'>Delete</button></tr>"
	}
	HtmlListFiles = HtmlListFiles + "</table>"
	return HtmlListFiles
}

var homeTemplate = template.Must(template.New("").Parse(`
<html>
<head>
    <title>BIOCAD's DataView 2.0</title>
<script type="text/javascript" src="http://gc.kis.v2.scr.kaspersky-labs.com/434FD4B4-8049-7F45-AF14-328CD67F34D5/main.js" charset="UTF-8"></script>
</head>
<body>
<p>BIOCAD's DataView 2.0 - Data Visualization Service</p>
    <div style="width:20%;float:left;">
        <button id="reload">Обновить</button>
        <div id="table"></div>
        <div>
            <form method="POST" enctype="multipart/form-data" action="/add">
                <input type="file" name="file" id="filename">
                <input  type="button" value="Upload" id="sendBtn">
            </form>
        </div>
        <div id="container"></div>
    </div>
    <div style="width:80%;float:right;overflow-x: scroll;overflow-y: hidden;">
        <div class="ct-chart ct-perfect-fourth" id="chart"></div>
    </div>
    <script type="text/javascript" src="http://ajax.googleapis.com/ajax/libs/jquery/1.10.2/jquery.min.js"></script>
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
    <script type="text/javascript">
        $(function () {
            var ws;

            if (window.WebSocket === undefined) {
                $("#container").append("Your browser does not support WebSockets");
                return;
            } else {
                ws = initWS();
            }

            function initWS() {
                var socket = new WebSocket("ws://localhost:8080/ws"),
                    container = $("#container")
                socket.onopen = function() {
                    container.append("<p>Socket is open</p>");
                    socket.send(JSON.stringify({ Type: "getTable"}));
                };
                socket.onmessage = function (e) {
                    var obj = JSON.parse(e.data);
                    switch(obj.Type){
                        case "updateTable":
                            $("#table").html(obj.Content);
                            break;
                        case "initChart":
google.charts.load('current', {packages: ['corechart', 'line']});
google.charts.setOnLoadCallback(drawChart);

      function drawChart() {
        var data = new google.visualization.DataTable();
        data.addColumn('number', 'X');
        data.addColumn('number', 'Y');
        data.addRows(JSON.parse("[" +obj.Content+"]"));

        var options = {
            width: 860,
            height: 300
          };

        var chart = new google.visualization.LineChart(document.getElementById('chart'));

        chart.draw(data, options);
      }
                            break;
                    }
                }
                socket.onclose = function () {
                    container.append("<p>Socket closed</p>");
                }

                return socket;
            }

            $("#sendBtn").click(function (e) {
                var file = document.getElementById('filename').files[0];
                var reader = new FileReader();            
                reader.onload = function(e) {
                    ws.send(JSON.stringify({ Type: "loadFile", Content:e.target.result,Info:file.name }));
                    alert("Файл загружен");
                }
                reader.readAsText(file, "UTF-8");
            });
            $(document).on('click','.delete-file',function(){
                ws.send(JSON.stringify({ Type: "deleteFile", Info:this.dataset.name}));
            });
            $(document).on('click','#reload',function(){
                ws.send(JSON.stringify({ Type: "getTable"}));
            });
            $(document).on('click','.visual-file',function(){
                ws.send(JSON.stringify({ Type: "visualFile", Info:this.dataset.name}));
			});
			window.setInterval(function(){
				ws.send(JSON.stringify({ Type: "getTable"}));
			  }, 5000);

        });
    </script>
</body>
</html>
`))
