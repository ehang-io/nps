(function ($) {
    $.fn.cloudLang = function (params) {

        var defaults = {
            file: '/static/page/lang-example.xml',
            lang: 'zh'
        }

        var aTexts = new Array();

        if (params) $.extend(defaults, params);

        $.ajax({
            type: "GET",
            url: defaults.file,
            dataType: "xml",
            success: function (xml) {
                $(xml).find('text').each(function () {
                    var textId = $(this).attr("id");
                    var text = $(this).find(defaults.lang).text();

                    aTexts[textId] = text;
                });

                $.each($("*"), function (i, item) {
                    //alert($(item).attr("langtag"));
                    if ($(item).attr("langtag") != null)
                        $(item).fadeOut(150).fadeIn(150).text(aTexts[$(item).attr("langtag")]);
                });
            }
        });
    };

})(jQuery);
$(document).ready(function () {
    function setCookie(c_name, value, expiredays) {
        var exdate = new Date()
        exdate.setDate(exdate.getDate() + expiredays)
        document.cookie = c_name + "=" + escape(value) +
            ((expiredays == null) ? "" : ";expires=" + exdate.toGMTString())
    }

    function getCookie(c_name) {
        if (document.cookie.length > 0) {
            c_start = document.cookie.indexOf(c_name + "=")
            if (c_start != -1) {
                c_start = c_start + c_name.length + 1
                c_end = document.cookie.indexOf(";", c_start)
                if (c_end == -1) c_end = document.cookie.length
                return unescape(document.cookie.substring(c_start, c_end))
            }
        }
        return ""
    }

    if (getCookie("lang") == "en") {
        $("body").cloudLang({lang: "en", file: "/static/page/lang-example.xml"});
    }
    $("#lang-en").click(function () {
        setCookie("lang", "en")
        $("body").cloudLang({lang: "en", file: "/static/page/lang-example.xml"});
    });

    $("#langzh").click(function () {
        setCookie("lang", "zh")
        $("body").cloudLang({lang: "zh", file: "/static/page/lang-example.xml"});
    });
});