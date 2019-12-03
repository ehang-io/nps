#/bash/sh
echo "start upgrading to the latest version"
if [ $1 == "latest" ]
then
  version=`wget -qO- -t1 -T2 "https://api.github.com/repos/cnlh/nps/releases/latest" | grep "tag_name" | head -n 1 | awk -F ":" '{print $2}' | sed 's/\"//g;s/,//g;s/ //g'`
else
  version=$1
fi
echo "the current latest version is "$version""
download_base_url=https://github.com/cnlh/nps/releases/download/$version/

if [ $4 ]
then
  filename=""$2"_"$3"_v"$4"_"server".tar.gz"
else
  filename=""$2"_"$3"_"server".tar.gz"
fi
complete_download_url=""$download_base_url""$filename""
echo "start download file from "$complete_download_url""

dir_name=`echo $RANDOM`
mkdir $dir_name && cd $dir_name
wget $complete_download_url >/dev/null 2>&1
if [ ! -f "$filename" ]; then
  echo "download file failed!"
  rm -rf $dir_name
  exit
fi

echo "start extracting files"
mkdir nps
tar -xvf $filename -C ./nps  >/dev/null 2>&1
cd nps

if [ -f "../../nps" ]; then
  echo "replace "../../nps"!"
  cp -rf nps ../../
fi

usr_dir=`which nps`

if [ -f "$usr_dir" ]; then
  echo "replace "$usr_dir"!"
  cp -rf nps $usr_dir
fi

cd ../../ && rm -rf $dir_name

echo "update complete!"
echo -e "\033[32m please restart nps \033[0m"
