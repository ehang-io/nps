/*
 * 后台页面的实时刷新支持。
 * 原来列表页只在首次加载时取一次数据：在别的窗口新增客户端/隧道、或者客户端上线后，
 * 当前页面要手动按 F5 才看得到，用户会以为没生效。这里统一做「静默轮询」：
 *  - 保留当前分页、每页条数、搜索词，不打断正在展开的详情行
 *  - 页面隐藏（切到别的标签页）时暂停，回到页面立即刷新一次，避免后台白跑请求
 */
(function (window, $) {
    'use strict';

    var DEFAULT_INTERVAL = 5000;

    // 生成一个「页面可见时才跑」的定时任务
    function runWhenVisible(fn, interval) {
        var timer = null;

        function start() {
            stop();
            timer = window.setInterval(function () {
                if (document.visibilityState === 'hidden') return;
                fn();
            }, interval);
        }

        function stop() {
            if (timer !== null) {
                window.clearInterval(timer);
                timer = null;
            }
        }

        document.addEventListener('visibilitychange', function () {
            if (document.visibilityState === 'visible') {
                fn();
                start();
            } else {
                stop();
            }
        });
        start();
        return { stop: stop, start: start };
    }

    /*
     * 让 bootstrap-table 定时静默刷新。
     * selector: 表格选择器；interval: 毫秒，0 表示不刷新
     */
    window.npsLiveTable = function (selector, interval) {
        var $table = $(selector);
        if ($table.length === 0) return null;
        var every = interval === undefined ? DEFAULT_INTERVAL : interval;
        if (!every || every <= 0) return null;

        var options = $table.bootstrapTable('getOptions');
        // 只刷新数据：不重建表格，不会重置分页和搜索框
        function refresh() {
            var state = $table.bootstrapTable('getOptions');
            $table.bootstrapTable('refresh', {
                silent: true,
                pageNumber: state.pageNumber,
                pageSize: state.pageSize,
                searchText: state.searchText
            });
        }
        if (options.search) {
            // 搜索输入时暂停轮询，否则会把用户正在打的字冲掉
            var searchTimer = null;
            $('.fixed-table-toolbar').on('input', 'input.search', function () {
                window.clearTimeout(searchTimer);
                searchTimer = window.setTimeout(refresh, 400);
            });
        }

        //供 submitform 在增删改成功后立即刷新一次，不必整页重载
        window.npsForceRefresh = refresh;
        //新增/编辑后按浏览器后退可能直接命中 bfcache，页面不会重新加载，
        //这里在 pageshow 时兜底刷一次数据
        window.addEventListener('pageshow', function (e) {
            if (e.persisted) refresh();
        });
        try {
            if (window.sessionStorage.getItem('nps:needRefresh') === '1') {
                window.sessionStorage.removeItem('nps:needRefresh');
                window.setTimeout(refresh, 0);
            }
        } catch (ignore) {
            /* 隐私模式下 sessionStorage 不可用，忽略即可 */
        }

        return runWhenVisible(refresh, every);
    };

    /*
     * 仪表盘轮询：更新 KPI 数字、系统负载条与饼图。
     * url: 返回 GetDashboardData 的 JSON 地址
     */
    window.npsLiveDashboard = function (url, interval) {
        if (!url) return null;
        var every = interval === undefined ? DEFAULT_INTERVAL : interval;
        var busy = false;

        function setText(id, text) {
            var el = document.getElementById(id);
            if (el) el.textContent = text;
        }

        function setWidth(id, percent) {
            var el = document.getElementById(id);
            if (el) el.style.width = percent + '%';
        }

        function apply(data) {
            setText('kpi_client_total', data.clientCount);
            setText('kpi_client_online', data.clientOnlineCount);
            setText('kpi_tcp_conn', data.tcpCount);

            setText('overview_cpu', data.cpu + '%');
            setWidth('overview_cpu_bar', data.cpu);
            setText('overview_memory', data.virtual_mem + '%');
            setWidth('overview_memory_bar', data.virtual_mem);
            setText('overview_tcp', data.tcp);
            setText('overview_udp', data.udp);
            if (typeof changeunit === 'function') {
                setText('overview_send', changeunit(data.io_send) + '/s');
                setText('overview_recv', changeunit(data.io_recv) + '/s');
            }

            // 饼图只更新数据，保留动画与图例
            if (window.charts && charts['flow']) {
                charts['flow'].setOption({
                    series: [{
                        data: [
                            { name: '', value: data.inletFlowCount },
                            { name: '', value: data.exportFlowCount }
                        ]
                    }]
                });
            }
            if (window.charts && charts['counts']) {
                charts['counts'].setOption({
                    series: [{
                        data: [
                            { name: '', value: data.hostCount },
                            { name: '', value: data.tcpC },
                            { name: '', value: data.udpCount },
                            { name: '', value: data.httpProxyCount },
                            { name: '', value: data.socks5Count },
                            { name: '', value: data.secretCount },
                            { name: '', value: data.p2pCount }
                        ]
                    }]
                });
            }
        }

        function poll() {
            if (busy) return;
            busy = true;
            $.getJSON(url, function (data) {
                apply(data);
            }).always(function () {
                busy = false;
            });
        }

        return runWhenVisible(poll, every);
    };
})(window, jQuery);
