jQuery(document).ready(function () {
                       // Set height
                       var content_height = $(".col-sm-9").height();
                       $(".col-sm-3").css("min-height", content_height + "px");
                       
                       
                       var url = window.location.pathname;
                       console.log(url);
                       //$("ul").find("a[href='" + url + "']").addClass('active');
                       var parents = $("ul").find("a[href='" + url + "']").parents('ul').length;
                       if (parents === 2) {
                       $("ul").find("a[href='" + url + "']").parent().parent().addClass('in');
                       $("ul").find("a[href='" + url + "']").parent().parent().parent().find('a.collapsed').attr('aria-expanded', 'true');
                       } else if (parents === 3) {
                       $("ul").find("a[href='" + url + "']").parent().parent().addClass('in');
                       var select = $("ul").find("a[href='" + url + "']").parent().parent().attr('id');
                       $("ul").find("a[href='" + url + "']").parent().parent().parent().parent().addClass('in');
                       $("ul").find("a[href='" + url + "']").parent().parent().parent().parent().parent().find('a[href=#' + select + ']').addClass('active');
                       var select = $("ul").find("a[href='" + url + "']").parent().parent().parent().parent().attr('id');
                       console.log(select);
                       $("ul").find("a[href='" + url + "']").parent().parent().parent().parent().parent().parent().find('a[href=#' + select + ']').addClass('active');
                       }
                       
                       var destination = $("ul.nav li a").attr("href");
                       $("ul.nav li a").click(function (event) {
                                              event.preventDefault();
                                              var link = $(this);
                                              var local_destination = link.attr("href");
                                              $.get(local_destination, function (data) {
                                                    var title = $(data).find("title").text();
                                                    var local_content = $(data).find(".col-sm-9").html();
                                                    changeUrl(title, local_destination);
                                                    checkActiveState();
                                                    $(".col-sm-9").html(local_content);
                                                    //If has a class hash (jump on the page)
                                                    if(link.hasClass('hash')){
                                                    var local_destination_array = local_destination.split('#');
                                                    var paragraph_id = local_destination_array[1];
                                                    var paragraph_destination = $('.col-sm-9 #' + paragraph_id).offset().top;
                                                    $('body,html').animate({scrollTop: paragraph_destination + "px"});
                                                    }
                                                    // Set height
                                                    var content_height = $(".col-sm-9").height();
                                                    $(".col-sm-3").css("min-height", content_height + "px");
                                                    });
                                              })
                       });
function changeUrl(title, url) {
    if (typeof (history.pushState) != "undefined") {
        var obj = {Title: title, Url: url};
        history.pushState(obj, obj.Title, obj.Url);
    }
}
function checkActiveState() {
    var url = window.location.pathname;
    $(".col-sm-3 ul .active").removeClass("active");
    $("ul").find("a[href='" + url + "']").addClass('active');
    $("ul").find("a[href='" + url + "']").closest('.list-group-submenu').parent().children('a').addClass("active");
}
