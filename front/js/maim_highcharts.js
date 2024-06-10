(async () => {
    try {
        const response = await fetch('/data');
        const data = await response.json();

        Highcharts.setOptions({
            lang: {
                loading: 'Загрузка...',
                months: ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'],
                weekdays: ['Воскресенье', 'Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота'],
                shortMonths: ['Янв', 'Фев', 'Март', 'Апр', 'Май', 'Июнь', 'Июль', 'Авг', 'Сент', 'Окт', 'Нояб', 'Дек'],
                exportButtonTitle: 'Экспорт',
                printButtonTitle: 'Печать',
                rangeSelectorFrom: 'С',
                rangeSelectorTo: 'По',
                rangeSelectorZoom: 'Период',
                downloadPNG: 'Скачать PNG',
                downloadJPEG: 'Скачать JPEG',
                downloadPDF: 'Скачать PDF',
                downloadSVG: 'Скачать SVG',
                printChart: 'Напечатать график',
                viewData: 'Показать таблицу данных'
            }
        });

        Highcharts.stockChart('containerq', {
            rangeSelector: {
                buttonTheme: {
                    width: 30 // Увеличим ширину кнопки
                },
                selected: 1,
                buttons: [{
                    type: 'month',
                    count: 1,
                    text: '1м'
                }, {
                    type: 'month',
                    count: 3,
                    text: '3м'
                }, {
                    type: 'month',
                    count: 6,
                    text: '6м'
                }, {
                    type: 'year',
                    count: 1,
                    text: '1г'
                }, {
                    type: 'all',
                    text: 'Все'
                }]
            },

            title: {
                text: 'Общая сумма заказов по времени'
            },

            subtitle: {
                text: 'Индивидуальный график'
            },

            scrollbar: {
                barBorderRadius: 0,
                barBorderWidth: 1,
                buttonsEnabled: true,
                height: 16,
                margin: 0,
                rifleColor: '#333',
                trackBackgroundColor: '#f2f2f2',
                trackBorderRadius: 0
            },

            series: [{
                name: 'Сумма прибыли',
                data: data,
                tooltip: {
                    valueDecimals: 2
                }
            }],

            exporting: {
                csv: {
                    dateFormat: '%d.%m.%Y'
                },
                table: {
                    dateFormat: '%d.%m.%Y %H:%M:%S'
                }
            },

            data: {
                dateFormat: '%d.%m.%Y'
            }
        });
    } catch (error) {
        console.error('Error loading data:', error);
    }
})();