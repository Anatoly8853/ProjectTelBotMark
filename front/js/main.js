document.addEventListener('DOMContentLoaded', function() {
    console.log('DOM content loaded');
    const ctx = document.getElementById('purchasesChart').getContext('2d');
    let chartType = 'line'; // Default chart type

    let chart = new Chart(ctx, {
        type: chartType,
        data: {
            labels: ['Янв', 'Фев', 'Март', 'Апр', 'Май', 'Июнь', 'Июль', 'Авг', 'Сент', 'Окт', 'Нояб', 'Дек'], // Метки для оси X
            datasets: [{
                label: 'Сумма покупок',
                data: [], // Данные для графика
                borderColor: 'rgba(75, 192, 192, 1)',
                backgroundColor: 'rgba(75, 192, 192, 0.2)',
                fill: true,
            }]
        },
        options: getChartOptions(chartType)
    });

    function getChartOptions(type) {
        if (type === 'pie' || type === 'doughnut') {
            return {
                plugins: {
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                return context.label + ': ' + context.raw;
                            }
                        }
                    }
                }
            };
        } else if (type === 'bar') {
            return {
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: 'Дата'
                        }
                    },
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Сумма покупок'
                        }
                    }
                }
            };
        } else {
            return {
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            parser: 'DD.MM.YYYY',
                            unit: 'day'
                        },
                        title: {
                            display: true,
                            text: 'Дата'
                        }
                    },
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Сумма покупок'
                        }
                    }
                }
            };
        }
    }

    function updateChart(period) {
        console.log('Updating chart for period:', period);
        fetch(`/purchases?period=${period}`)
            .then(response => response.json())
            .then(data => {
                chart.data.labels = data.labels;
                chart.data.datasets[0].data = data.amounts;

                if (chart.options.scales && chart.options.scales.x && chart.options.scales.x.time) {
                    switch (period) {
                        case 'day':
                            chart.options.scales.x.time.unit = 'hour';
                            break;
                        case 'week':
                            chart.options.scales.x.time.unit = 'day';
                            break;
                        case 'month':
                            chart.options.scales.x.time.unit = 'week';
                            break;
                        case 'year':
                            chart.options.scales.x.time.unit = 'month';
                            break;
                    }
                }
                chart.update();
            });
    }

    function recreateChart(newType) {
        chart.destroy();
        chart = new Chart(ctx, {
            type: newType,
            data: chart.data,
            options: getChartOptions(newType)
        });

        // Re-fetch the data to update the chart with the new type
        const period = document.getElementById('timePeriod').value;
        updateChart(period);
    }

    document.getElementById('timePeriod').addEventListener('change', function() {
        const period = this.value;
        updateChart(period);
    });

    document.getElementById('chartType').addEventListener('change', function() {
        const newType = this.value;
        recreateChart(newType);
    });

    // Инициализируем график с данными за день
    updateChart('day');
});
